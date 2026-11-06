// SPDX-License-Identifier: GPL-3.0-or-later
package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// A realistic upload: the latest value plus three finished one-minute buckets, as one multi-path update.
func sampleUpdate() map[string]any {
	rec := func(extra map[string]any) map[string]any {
		return record(sample{CPU: 37.52, Load1: 1.23, MemUsed: 8123456789, MemTotal: 17179869184, DiskUsed: 412345678902, DiskTotal: 1e12, NetRx: 123457, NetTx: 7654}, extra)
	}
	u := map[string]any{"latest/dev1": rec(map[string]any{"ts": 1, "iv": 180, "ut": 98765, "up": 123456, "down": 7890})}
	for i := 0; i < 3; i++ {
		u[fmt.Sprintf("m/dev1/20261106/%d", 1793280000+i*60)] = rec(map[string]any{"t": 1793280000 + i*60})
	}
	return u
}

type seen struct {
	gzipped, plain int
	rawBytes       []int // request body sizes as they crossed the wire
	decoded        []map[string]json.RawMessage
}

func testClient(h http.HandlerFunc, compress bool) (*fbClient, *httptest.Server) {
	srv := httptest.NewServer(h)
	fc := newFBClient(srv.Client(), &fbSaved{APIKey: "k", DatabaseURL: srv.URL, UID: "dev1"})
	fc.idToken, fc.expiry, fc.now = "tok", time.Now().Add(24*time.Hour), time.Now
	fc.compress = compress
	return fc, srv
}

func TestCompressedUploadsAreSmallerAndIntact(t *testing.T) {
	run := func(compress bool) *seen {
		got := &seen{}
		fc, srv := testClient(func(w http.ResponseWriter, r *http.Request) {
			raw, _ := io.ReadAll(r.Body)
			got.rawBytes = append(got.rawBytes, len(raw))
			body := raw
			if r.Header.Get("Content-Encoding") == "gzip" {
				got.gzipped++
				zr, err := gzip.NewReader(bytesReader(raw))
				if err != nil {
					http.Error(w, "bad gzip", http.StatusBadRequest)
					return
				}
				body, _ = io.ReadAll(zr)
			} else {
				got.plain++
			}
			var m map[string]json.RawMessage
			if err := json.Unmarshal(body, &m); err != nil {
				http.Error(w, "bad json", http.StatusBadRequest)
				return
			}
			got.decoded = append(got.decoded, m)
			w.WriteHeader(http.StatusNoContent)
		}, compress)
		defer srv.Close()
		if _, err := fc.db("PATCH", "", sampleUpdate(), nil); err != nil {
			t.Fatal(err)
		}
		return got
	}
	plain, packed := run(false), run(true)
	if packed.gzipped != 1 || plain.gzipped != 0 {
		t.Fatalf("expected exactly the compressed run to send gzip (plain=%d packed=%d)", plain.gzipped, packed.gzipped)
	}
	if len(packed.decoded[0]) != len(plain.decoded[0]) {
		t.Fatalf("the server must receive the same data either way: %d vs %d paths", len(packed.decoded[0]), len(plain.decoded[0]))
	}
	t.Logf("upload body: %d bytes plain, %d bytes gzipped (%.0f%% smaller)", plain.rawBytes[0], packed.rawBytes[0], 100*(1-float64(packed.rawBytes[0])/float64(plain.rawBytes[0])))
	if packed.rawBytes[0] > plain.rawBytes[0]/2 {
		t.Fatalf("gzip should at least halve a body of repeated keys: %d -> %d", plain.rawBytes[0], packed.rawBytes[0])
	}
}

// If the database refuses compressed bodies, nothing may be lost and compression must stay off afterwards.
func TestCompressionFallsBackWhenTheServerRefusesIt(t *testing.T) {
	got := &seen{}
	fc, srv := testClient(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		if r.Header.Get("Content-Encoding") != "" {
			got.gzipped++
			http.Error(w, "Invalid data; couldn't parse JSON object", http.StatusBadRequest)
			return
		}
		got.plain++
		w.WriteHeader(http.StatusNoContent)
	}, true)
	defer srv.Close()

	if _, err := fc.db("PATCH", "", sampleUpdate(), nil); err != nil {
		t.Fatalf("the upload must still succeed by resending plain: %v", err)
	}
	if fc.compress {
		t.Fatal("compression should be switched off after the server refused it")
	}
	if got.gzipped != 1 || got.plain != 1 {
		t.Fatalf("expected one refused gzip attempt then one plain resend, got gzip=%d plain=%d", got.gzipped, got.plain)
	}
	if _, err := fc.db("PATCH", "", sampleUpdate(), nil); err != nil {
		t.Fatal(err)
	}
	if got.gzipped != 1 || got.plain != 2 {
		t.Fatalf("after the fallback no more gzip attempts are allowed: gzip=%d plain=%d", got.gzipped, got.plain)
	}
}

func bytesReader(b []byte) io.Reader { return bytes.NewReader(b) }
