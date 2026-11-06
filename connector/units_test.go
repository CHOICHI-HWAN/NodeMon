// SPDX-License-Identifier: GPL-3.0-or-later
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestParseOptions(t *testing.T) {
	const db = "https://demo-default-rtdb.asia-southeast1.firebasedatabase.app"
	cases := []struct {
		name    string
		args    []string
		wantErr string // substring of the error, "" for success
	}{
		{"defaults", nil, ""},
		{"enroll with firebase", []string{"--code", "AAAA", "--firebase-api-key", "k", "--firebase-db-url", db}, ""},
		{"enroll against the local emulator over plain http", []string{"--code", "AAAA", "--firebase-api-key", "k", "--firebase-db-url", "http://127.0.0.1:9000?ns=x"}, ""},
		{"enroll with a self-hosted server on the LAN", []string{"--code", "AAAA", "--server", "http://192.168.0.5:8080"}, ""},
		{"zero interval would spin", []string{"--interval", "0"}, "--interval must be at least 1s"},
		{"sub-second interval", []string{"--interval", "500ms"}, "--interval must be at least 1s"},
		{"upload interval beyond what the rules accept", []string{"--upload-every", "25h"}, "--upload-every must be between"},
		{"upload interval too small", []string{"--upload-every", "0s"}, "--upload-every must be between"},
		{"code without a backend", []string{"--code", "AAAA"}, "needs either --server or --firebase-db-url"},
		{"firebase url without a key", []string{"--code", "AAAA", "--firebase-db-url", db}, "--firebase-api-key is required"},
		{"login token over plain http to another host", []string{"--code", "AAAA", "--firebase-api-key", "k", "--firebase-db-url", "http://db.example.com"}, "must start with https://"},
		{"not a url", []string{"--code", "AAAA", "--firebase-api-key", "k", "--firebase-db-url", "nonsense"}, "not a valid address"},
		{"insecure auth override", []string{"--code", "AAAA", "--firebase-api-key", "k", "--firebase-db-url", db, "--firebase-auth-url", "http://auth.example.com"}, "--firebase-auth-url"},
		{"version needs nothing else", []string{"--version", "--interval", "0"}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := parseOptions(c.args, io.Discard)
			switch {
			case c.wantErr == "" && err != nil:
				t.Fatalf("unexpected error: %v", err)
			case c.wantErr != "" && (err == nil || !strings.Contains(err.Error(), c.wantErr)):
				t.Fatalf("error = %v, want one containing %q", err, c.wantErr)
			}
		})
	}
	if _, err := parseOptions([]string{"--no-such-flag"}, io.Discard); !errors.Is(err, errBadFlags) {
		t.Fatalf("an unknown flag should be reported as a flag error, got %v", err)
	}
	if _, err := parseOptions([]string{"-h"}, io.Discard); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("-h should ask for help, got %v", err)
	}
}

func at(h, m, s int) time.Time { return time.Date(2026, 11, 6, h, m, s, 0, time.UTC) }

func TestBucketerClosesMinutesAndHours(t *testing.T) {
	b := bucketer{uid: "dev"}
	feed := func(now time.Time, cpu float64) []finished {
		out := b.roll(now)
		b.add(sample{CPU: cpu, MemTotal: 2})
		return out
	}
	if got := feed(at(10, 5, 10), 10); len(got) != 0 {
		t.Fatalf("nothing has finished yet: %v", got)
	}
	if got := feed(at(10, 5, 40), 30); len(got) != 0 {
		t.Fatalf("same minute, still nothing: %v", got)
	}
	got := feed(at(10, 6, 5), 50) // the 10:05 minute is over
	if len(got) != 1 || got[0].hour || got[0].rec["cpu"] != 20.0 {
		t.Fatalf("expected the 10:05 minute averaged to 20, got %+v", got)
	}
	wantPath := fmt.Sprintf("m/dev/20261106/%d", at(10, 5, 0).Unix())
	if got[0].path != wantPath {
		t.Fatalf("path = %s, want %s", got[0].path, wantPath)
	}
	got = feed(at(11, 0, 5), 70) // 10:06 ends, and with it the 10:00 hour
	if len(got) != 2 || !got[1].hour {
		t.Fatalf("expected a minute and an hour bucket, got %+v", got)
	}
	if got[1].rec["t"] != at(10, 0, 0).Unix() {
		t.Fatalf("the hour bucket should start at 10:00, got %v", got[1].rec["t"])
	}
	if !strings.HasPrefix(got[1].path, "h/dev/202611/") {
		t.Fatalf("hour buckets are grouped by month: %s", got[1].path)
	}
}

func TestBucketerGroupsMinutesByUTCDay(t *testing.T) {
	b := bucketer{uid: "dev"}
	b.roll(time.Date(2026, 11, 6, 23, 59, 30, 0, time.UTC))
	b.add(sample{CPU: 1})
	got := b.roll(time.Date(2026, 11, 7, 0, 0, 5, 0, time.UTC))
	if len(got) < 1 || !strings.Contains(got[0].path, "/20261106/") {
		t.Fatalf("the 23:59 minute belongs to the day it started in: %+v", got)
	}
}

func TestBackoff(t *testing.T) {
	cases := []struct {
		base  time.Duration
		fails int
		want  time.Duration
	}{
		{3 * time.Minute, 0, 3 * time.Minute},
		{3 * time.Minute, 1, 6 * time.Minute},
		{3 * time.Minute, 2, 12 * time.Minute},
		{3 * time.Minute, 3, 15 * time.Minute}, // capped
		{3 * time.Minute, 100, 15 * time.Minute},
		{20 * time.Minute, 0, 20 * time.Minute}, // an interval above the cap is never shortened
		{20 * time.Minute, 5, 20 * time.Minute},
	}
	for _, c := range cases {
		if got := (&backoff{base: c.base, fails: c.fails}).wait(); got != c.want {
			t.Errorf("base %s after %d failures: wait %s, want %s", c.base, c.fails, got, c.want)
		}
	}
}

func TestTrimQueueDropsTheOldestMinutes(t *testing.T) {
	s := &fbSink{queue: map[string]any{}}
	for i := 0; i < maxQueued+100; i++ {
		s.queue[fmt.Sprintf("m/dev/20261106/%d", 1793280000+i*60)] = i
	}
	s.queue["h/dev/202611/1793278800"] = "hour"
	s.trimQueue()
	if len(s.queue) != maxQueued {
		t.Fatalf("queue has %d entries, want %d", len(s.queue), maxQueued)
	}
	if _, ok := s.queue["h/dev/202611/1793278800"]; !ok {
		t.Fatal("hour buckets must survive; only old minute buckets are dropped")
	}
	if _, ok := s.queue[fmt.Sprintf("m/dev/20261106/%d", 1793280000)]; ok {
		t.Fatal("the oldest minute should have been dropped")
	}
	newest := fmt.Sprintf("m/dev/20261106/%d", 1793280000+(maxQueued+99)*60)
	if _, ok := s.queue[newest]; !ok {
		t.Fatal("the newest minute must be kept")
	}
}

func TestCredStoreSavesOnRotationAndOnDemand(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "connector.json")
	cfg := &config{Backend: "firebase", Firebase: &fbSaved{RefreshToken: "r1"}}
	usage := newUsageCounter(0, 0)
	clock := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s := newCredStore(path, cfg, usage)
	s.now = func() time.Time { return clock }
	s.lastWrite = clock

	usage.up.Add(500)
	s.save(false)
	if _, err := os.Stat(path); err == nil {
		t.Fatal("usage grew but a minute has not passed: the file must not be rewritten yet")
	}
	clock = clock.Add(61 * time.Second)
	s.save(false)
	if got, err := loadConfig(path); err != nil || got.Usage == nil || got.Usage.Up != 500 {
		t.Fatalf("after a minute the total should be saved: %+v %v", got.Usage, err)
	}
	usage.up.Add(100)
	cfg.Firebase.RefreshToken = "r2" // Firebase rotated it: that must be saved at once
	s.save(false)
	if got, _ := loadConfig(path); got.Firebase.RefreshToken != "r2" || got.Usage.Up != 600 {
		t.Fatalf("a rotated token must be saved immediately: %+v", got.Firebase)
	}
	usage.up.Add(7)
	s.save(true) // shutting down
	if got, _ := loadConfig(path); got.Usage.Up != 607 {
		t.Fatalf("a forced save must write the latest total, got %d", got.Usage.Up)
	}
	if runtime.GOOS != "windows" {
		if st, _ := os.Stat(path); st.Mode().Perm() != 0o600 {
			t.Fatalf("credentials must be owner-only, got %v", st.Mode().Perm())
		}
	}
}

func TestQuietLogRepeatsOnlyOncePerInterval(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)
	clock := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	q := newQuietLog(10 * time.Minute)
	q.now = func() time.Time { return clock }

	for i := 0; i < 5; i++ { // a stopped Netdata, checked every ten seconds
		q.problem("netdata", "netdata: connection refused")
		clock = clock.Add(10 * time.Second)
	}
	if n := strings.Count(buf.String(), "connection refused"); n != 1 {
		t.Fatalf("the same problem was printed %d times within one interval, want 1", n)
	}
	clock = clock.Add(11 * time.Minute)
	q.problem("netdata", "netdata: connection refused")
	if n := strings.Count(buf.String(), "connection refused"); n != 2 {
		t.Fatalf("after the interval it should be reminded once more, got %d", n)
	}
	q.recovered("netdata", "netdata: back")
	q.recovered("netdata", "netdata: back") // only said once
	if n := strings.Count(buf.String(), "netdata: back"); n != 1 {
		t.Fatalf("recovery should be announced once, got %d", n)
	}
	q.problem("send", "send: boom") // a different problem is not suppressed by the first
	if !strings.Contains(buf.String(), "send: boom") {
		t.Fatal("independent problems must each be printed")
	}
}

func TestExplainAuthError(t *testing.T) {
	cases := map[string]string{
		`{"error":{"message":"ADMIN_ONLY_OPERATION"}}`:        "Authentication > Sign-in method > Anonymous",
		`{"error":{"message":"OPERATION_NOT_ALLOWED"}}`:       "Authentication > Sign-in method > Anonymous",
		`{"error":{"message":"API key not valid. Please"}}`:   "--firebase-api-key",
		`{"error":{"message":"TOO_MANY_ATTEMPTS_TRY_LATER"}}`: "Wait a few minutes",
		`{"error":{"message":"INVALID_REFRESH_TOKEN"}}`:       "Register it again",
	}
	for body, want := range cases {
		if got := explainAuthError("400 Bad Request", body); !strings.Contains(got, want) {
			t.Errorf("%s -> %q, want it to mention %q", body, got, want)
		}
	}
	long := strings.Repeat("x", 1000)
	if got := explainAuthError("500", long); len(got) > 330 {
		t.Errorf("unknown errors are shortened, got %d characters", len(got))
	}
}
