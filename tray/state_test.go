// SPDX-License-Identifier: GPL-3.0-or-later
package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestEvaluate(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	at := func(d time.Duration) int64 { return now.Add(d).Unix() }
	cases := []struct {
		name string
		s    *status
		err  error
		want state
	}{
		{"no file", nil, errors.New("missing"), stateStopped},
		{"connector stopped (stale file)", &status{Updated: at(-5 * time.Minute), LastSent: at(-6 * time.Minute), UploadSecs: 180}, nil, stateStopped},
		{"just started", &status{Updated: at(-5 * time.Second), UploadSecs: 180}, nil, stateStarting},
		{"healthy", &status{Updated: at(-10 * time.Second), LastSent: at(-90 * time.Second), UploadSecs: 180}, nil, stateOK},
		{"upload late", &status{Updated: at(-10 * time.Second), LastSent: at(-9 * time.Minute), UploadSecs: 180}, nil, stateProblem},
		{"problem reported", &status{Updated: at(-10 * time.Second), LastSent: at(-60 * time.Second), UploadSecs: 180, Problem: "send: denied"}, nil, stateProblem},
	}
	for _, c := range cases {
		got, line1, _ := evaluate(c.s, c.err, now)
		if got != c.want {
			t.Errorf("%s: state = %v, want %v (%s)", c.name, got, c.want, line1)
		}
	}
}

func TestMakeIconIsPNG(t *testing.T) {
	for st, c := range colors {
		b := makeIcon(c)
		if len(b) < 100 || string(b[1:4]) != "PNG" {
			t.Errorf("state %v: not a PNG (%d bytes)", st, len(b))
		}
	}
}

func TestPickStatusPrefersTheFreshestFile(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	old := write("old.json", `{"updated": 100, "upload_secs": 180}`)
	fresh := write("fresh.json", `{"updated": 900, "upload_secs": 180}`)
	if s, err := pickStatus([]string{old, filepath.Join(dir, "missing.json"), fresh}); err != nil || s.Updated != 900 {
		t.Errorf("got %+v, %v", s, err)
	}
	if _, err := pickStatus([]string{filepath.Join(dir, "missing.json")}); err == nil {
		t.Error("no file must be an error")
	}
	if _, err := pickStatus(nil); err == nil {
		t.Error("no candidates must be an error")
	}
}
