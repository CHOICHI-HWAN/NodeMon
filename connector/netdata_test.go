// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
)

const linuxFixture = `{
 "system.cpu": {"dimensions": {"user": {"value": 10}, "system": {"value": 5}, "idle": {"value": 85}}},
 "system.ram": {"dimensions": {"free": {"value": 1000}, "used": {"value": 3000}, "cached": {"value": 2000}, "buffers": {"value": 2000}}},
 "system.net": {"dimensions": {"received": {"value": 800}, "sent": {"value": -400}}},
 "system.load": {"dimensions": {"load1": {"value": 0.5}, "load5": {"value": 0.4}}},
 "disk_space._": {"dimensions": {"avail": {"value": 60}, "used": {"value": 30}, "reserved_for_root": {"value": 10}}},
 "disk_space._boot": {"dimensions": {"avail": {"value": 0.5}, "used": {"value": 0.2}, "reserved_for_root": {"value": 0}}}
}`

const macFixture = `{
 "system.cpu": {"dimensions": {"user": {"value": 20}, "idle": {"value": 80}}},
 "system.ram": {"dimensions": {"free": {"value": 1000}, "active": {"value": 3000}, "inactive": {"value": 2000}, "wired": {"value": 2000}}}
}`

func near(a, b float64) bool { return math.Abs(a-b) < 1e-6 }

func parse(t *testing.T, s string) sample {
	t.Helper()
	var charts map[string]ndChart
	if err := json.Unmarshal([]byte(s), &charts); err != nil {
		t.Fatal(err)
	}
	out, err := toSample(charts)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestLinux(t *testing.T) {
	s := parse(t, linuxFixture)
	if !near(s.CPU, 15) {
		t.Errorf("cpu = %v", s.CPU)
	}
	if !near(s.MemTotal, 8000*mib) || !near(s.MemUsed, 3000*mib) {
		t.Errorf("mem = %v/%v", s.MemUsed, s.MemTotal)
	}
	if !near(s.NetRx, 100000) || !near(s.NetTx, 50000) { // kbit/s -> bytes/s, sent made positive
		t.Errorf("net = %v/%v", s.NetRx, s.NetTx)
	}
	if !near(s.DiskTotal, 100*gib) || !near(s.DiskUsed, 30*gib) {
		t.Errorf("disk = %v/%v", s.DiskUsed, s.DiskTotal)
	}
	if !near(s.Load1, 0.5) {
		t.Errorf("load1 = %v", s.Load1)
	}
}

func TestMacRamFallbackAndMissingCharts(t *testing.T) {
	s := parse(t, macFixture)
	if !near(s.MemUsed, 5000*mib) { // total 8000 - free 1000 - inactive 2000
		t.Errorf("mem used = %v", s.MemUsed)
	}
	if s.DiskTotal != 0 || s.NetRx != 0 || s.Load1 != 0 {
		t.Errorf("missing charts should be zero: %+v", s)
	}
}

func TestNoCPUChart(t *testing.T) {
	if _, err := toSample(map[string]ndChart{}); err == nil {
		t.Error("expected error")
	}
}

const macNetFixture = `{
 "system.cpu": {"dimensions": {"idle": {"value": 80}}},
 "net.en0": {"dimensions": {"received": {"value": 800}, "sent": {"value": -400}}},
 "net.en1": {"dimensions": {"received": {"value": 80}, "sent": {"value": 0}}},
 "net.lo0": {"dimensions": {"received": {"value": 9999}, "sent": {"value": -9999}}},
 "net.utun3": {"dimensions": {"received": {"value": 9999}, "sent": {"value": -9999}}}
}`

// macOS has no system.net: the physical interfaces are added up and tunnels/loopback are left out.
func TestNetFromInterfacesWhenNoTotal(t *testing.T) {
	s := parse(t, macNetFixture)
	if !near(s.NetRx, 110000) || !near(s.NetTx, 50000) {
		t.Errorf("net = %v/%v", s.NetRx, s.NetTx)
	}
}

func TestParseNetAverage(t *testing.T) {
	rx, tx, ok := parseNetAverage(strings.NewReader(`{"labels":["time","received","sent"],"data":[[1794317260,800,-400]]}`))
	if !ok || !near(rx, 100000) || !near(tx, 50000) {
		t.Errorf("got %v/%v ok=%v", rx, tx, ok)
	}
	if _, _, ok := parseNetAverage(strings.NewReader(`{"labels":["time","received","sent"],"data":[]}`)); ok {
		t.Error("an empty answer must not count as a reading")
	}
}
