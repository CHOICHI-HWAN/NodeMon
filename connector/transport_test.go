// SPDX-License-Identifier: GPL-3.0-or-later
package main

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// secondUploadBytes sends two identical uploads gap apart over HTTP/2 and returns what the second one cost.
// The URL carries a long token, like a Firebase request, so header compression matters.
func secondUploadBytes(t *testing.T, idle, gap time.Duration) (bytesForSecond int64, proto int) {
	t.Helper()
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusNoContent)
	}))
	srv.EnableHTTP2 = true
	srv.StartTLS()
	defer srv.Close()
	pool := x509.NewCertPool()
	pool.AddCert(srv.Certificate())

	u := newUsageCounter(0, 0)
	client := newCountingClient(u, idle)
	client.Transport.(*http.Transport).TLSClientConfig = &tls.Config{RootCAs: pool}

	url := srv.URL + "/.json?auth=" + strings.Repeat("A1b2C3d4E5", 77) + "&print=silent" // 770-character token
	payload := []byte(strings.Repeat(`{"cpu":37.52}`, 40))
	send := func() int {
		resp, err := client.Do(mustPatch(t, url, payload))
		if err != nil {
			t.Fatal(err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		return resp.ProtoMajor
	}
	proto = send()
	before, _ := u.snapshot()
	time.Sleep(gap)
	send()
	after, _ := u.snapshot()
	return after - before, proto
}

func mustPatch(t *testing.T, url string, body []byte) *http.Request {
	t.Helper()
	req, err := http.NewRequest(http.MethodPatch, url, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	return req
}

// Uploads are minutes apart, so the connection has to survive the gap; otherwise every upload pays for a
// new TLS handshake and sends the whole token again.
func TestConnectionIsKeptBetweenUploads(t *testing.T) {
	reused, proto := secondUploadBytes(t, 5*time.Second, 600*time.Millisecond)
	if proto != 2 {
		t.Fatalf("expected HTTP/2 (header compression depends on it), got HTTP/%d", proto)
	}
	dropped, _ := secondUploadBytes(t, 200*time.Millisecond, 600*time.Millisecond) // idle timeout shorter than the gap
	t.Logf("second upload: %d bytes on a kept connection, %d bytes when the connection had been closed", reused, dropped)
	if reused > 800 {
		t.Fatalf("a repeated upload on a kept HTTP/2 connection should be small, got %d bytes", reused)
	}
	if dropped < reused*3 {
		t.Fatalf("closing the connection should cost clearly more (handshake and headers): kept %d, closed %d", reused, dropped)
	}
}
