// SPDX-License-Identifier: GPL-3.0-or-later
package main

import (
	"encoding/json"
	"runtime"
	"testing"
	"time"
)

func TestUptimeFromNetdataWins(t *testing.T) {
	var charts map[string]ndChart
	json.Unmarshal([]byte(`{
	  "system.cpu": {"dimensions": {"idle": {"value": 90}}},
	  "system.uptime": {"dimensions": {"uptime": {"value": 123456}}}
	}`), &charts)
	s, err := toSample(charts)
	if err != nil {
		t.Fatal(err)
	}
	if s.Uptime != 123456 {
		t.Fatalf("uptime = %v, want Netdata's 123456", s.Uptime)
	}
	withUptime(&s) // must not replace a value Netdata already gave us
	if s.Uptime != 123456 {
		t.Fatalf("the operating-system fallback overwrote Netdata's value: %v", s.Uptime)
	}
}

func TestUptimeFallsBackToTheOperatingSystem(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("this check reads /proc/uptime")
	}
	s := sample{} // Netdata reported nothing
	withUptime(&s)
	if s.Uptime <= 0 {
		t.Fatalf("expected the fallback to read /proc/uptime, got %v", s.Uptime)
	}
}

func TestParseBoottime(t *testing.T) {
	now := time.Unix(1730003600, 0)
	got, ok := parseBoottime("{ sec = 1730000000, usec = 123456 } Sun Oct 27 04:53:20 2024\n", now)
	if !ok || got != 3600 {
		t.Fatalf("got %v, %v; want 3600 seconds", got, ok)
	}
	for _, bad := range []string{"", "garbage", "{ sec = 9999999999, usec = 0 }"} { // empty, no number, a boot time in the future
		if _, ok := parseBoottime(bad, now); ok {
			t.Fatalf("%q should not parse", bad)
		}
	}
}
