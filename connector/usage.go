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
func newCountingClient(u *usageCounter) *http.Client {
	d := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
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
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: time.Second,
	}
	return &http.Client{Transport: tr, Timeout: 10 * time.Second}
}
