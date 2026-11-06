// SPDX-License-Identifier: GPL-3.0-or-later
package main

import (
	"context"
	"net"
	"net/http"
	"sync/atomic"
	"time"
)

// usageCounter totals the bytes this connector has really sent to and received from Firebase.
// It counts at the socket, below TLS and HTTP/2, so encryption and protocol overhead are included
// (IP and TCP headers are not). Only the Firebase client uses it; reading the local Netdata agent does not count.
type usageCounter struct {
	up, down atomic.Int64
}

func newUsageCounter(up, down int64) *usageCounter {
	u := &usageCounter{}
	u.up.Store(up)
	u.down.Store(down)
	return u
}

func (u *usageCounter) snapshot() (up, down int64) { return u.up.Load(), u.down.Load() }

type countConn struct {
	net.Conn
	u *usageCounter
}

func (c *countConn) Read(b []byte) (int, error) {
	n, err := c.Conn.Read(b)
	c.u.down.Add(int64(n))
	return n, err
}

func (c *countConn) Write(b []byte) (int, error) {
	n, err := c.Conn.Write(b)
	c.u.up.Add(int64(n))
	return n, err
}

// newCountingClient returns an HTTP client whose every connection feeds the counter.
// idle is how long an unused connection is kept. It must be longer than the gap between uploads: a new
// connection costs a TLS handshake (about 2.4 KB) and full-size request headers, while a reused HTTP/2
// connection sends the repeated headers as a few bytes.
func newCountingClient(u *usageCounter, idle time.Duration) *http.Client {
	d := &net.Dialer{
		Timeout: 10 * time.Second,
		// Probe rarely: each probe is a packet on the wire, and a connection that stays open is the point.
		KeepAliveConfig: net.KeepAliveConfig{Enable: true, Idle: 2 * time.Minute, Interval: 2 * time.Minute, Count: 3},
	}
	tr := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			c, err := d.DialContext(ctx, network, addr)
			if err != nil {
				return nil, err
			}
			return &countConn{Conn: c, u: u}, nil
		},
		ForceAttemptHTTP2:     true, // a custom dialer would otherwise turn HTTP/2 off
		MaxIdleConns:          4,
		IdleConnTimeout:       idle,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: time.Second,
	}
	return &http.Client{Transport: tr, Timeout: 10 * time.Second}
}
