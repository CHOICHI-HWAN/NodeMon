// SPDX-License-Identifier: GPL-3.0-or-later
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStatusWriter(t *testing.T) {
	dir := t.TempDir()
	clock := time.Unix(1_800_000_000, 0)
	w := newStatusWriter(filepath.Join(dir, "connector.json"), 3*time.Minute)
	w.now = func() time.Time { return clock }
	read := func() connectorStatus {
		var s connectorStatus
		b, err := os.ReadFile(filepath.Join(dir, "status.json"))
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(b, &s); err != nil {
			t.Fatal(err)
		}
		return s
	}

	sent := clock.Add(-time.Minute)
	w.write(sent, "")
	if s := read(); s.LastSent != sent.Unix() || s.Problem != "" || s.UploadSecs != 180 || s.Updated != clock.Unix() {
		t.Fatalf("first write = %+v", s)
	}

	clock = clock.Add(10 * time.Second)
	w.write(sent, "") // nothing changed and the refresh time has not passed: no new write
	if s := read(); s.Updated == clock.Unix() {
		t.Error("an unchanged status was rewritten within the refresh time")
	}

	w.write(sent, "send: boom") // a change is written at once
	if s := read(); s.Problem != "send: boom" || s.Updated != clock.Unix() {
		t.Errorf("change not written: %+v", s)
	}

	clock = clock.Add(31 * time.Second)
	w.write(sent, "send: boom") // still the same, but old enough to tell the icon the connector is alive
	if s := read(); s.Updated != clock.Unix() {
		t.Errorf("heartbeat missing: %+v", s)
	}
	if _, err := os.Stat(filepath.Join(dir, "status.json.tmp")); err == nil {
		t.Error("temporary file left behind")
	}
}

func TestStatusWriterNilIsSafe(t *testing.T) {
	var w *statusWriter
	w.write(time.Now(), "") // health values built without a writer (tests) must not crash
}
