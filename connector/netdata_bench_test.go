// SPDX-License-Identifier: GPL-3.0-or-later
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
)

// bigAllmetrics builds a payload shaped like a busy Netdata host: hundreds of charts, most of them
// irrelevant to us, plus the handful of system charts and mounted filesystems that we do read.
func bigAllmetrics(charts int) []byte {
	m := map[string]any{}
	dims := func(n int) map[string]any {
		d := map[string]any{}
		for i := 0; i < n; i++ {
			id := fmt.Sprintf("dim_%d", i)
			d[id] = map[string]any{"name": id, "value": float64(i) * 1.2345}
		}
		return d
	}
	for i := 0; i < charts; i++ {
		id := fmt.Sprintf("app.proc_%d.cpu", i)
		m[id] = map[string]any{"name": id, "family": "processes", "context": "app.cpu", "units": "percentage", "last_updated": 1730000000, "dimensions": dims(20)}
	}
	m["system.cpu"] = map[string]any{"name": "system.cpu", "units": "percentage", "last_updated": 1730000000, "dimensions": map[string]any{"user": map[string]any{"name": "user", "value": 10.0}, "idle": map[string]any{"name": "idle", "value": 85.0}}}
	m["system.ram"] = map[string]any{"dimensions": map[string]any{"free": map[string]any{"value": 1000.0}, "used": map[string]any{"value": 3000.0}, "cached": map[string]any{"value": 2000.0}, "buffers": map[string]any{"value": 500.0}}}
	m["system.net"] = map[string]any{"dimensions": map[string]any{"received": map[string]any{"value": 800.0}, "sent": map[string]any{"value": -400.0}}}
	m["system.load"] = map[string]any{"dimensions": map[string]any{"load1": map[string]any{"value": 0.5}}}
	m["system.uptime"] = map[string]any{"dimensions": map[string]any{"uptime": map[string]any{"value": 123456.0}}}
	m["disk_space._"] = map[string]any{"dimensions": map[string]any{"avail": map[string]any{"value": 60.0}, "used": map[string]any{"value": 30.0}, "reserved_for_root": map[string]any{"value": 10.0}}}
	b, _ := json.Marshal(m)
	return b
}

func BenchmarkDecodeEverything(b *testing.B) {
	payload := bigAllmetrics(400)
	b.SetBytes(int64(len(payload)))
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var charts map[string]ndChart
		if err := json.NewDecoder(bytes.NewReader(payload)).Decode(&charts); err != nil {
			b.Fatal(err)
		}
		if _, err := toSample(charts); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkReadOnlyWhatWeUse(b *testing.B) {
	payload := bigAllmetrics(400)
	b.SetBytes(int64(len(payload)))
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		charts, err := readCharts(bytes.NewReader(payload))
		if err != nil {
			b.Fatal(err)
		}
		if _, err := toSample(charts); err != nil {
			b.Fatal(err)
		}
	}
}

// The shortcut must give exactly the answer the full decode gives.
func TestReadChartsMatchesFullDecode(t *testing.T) {
	payload := bigAllmetrics(50)
	var all map[string]ndChart
	if err := json.Unmarshal(payload, &all); err != nil {
		t.Fatal(err)
	}
	want, err := toSample(all)
	if err != nil {
		t.Fatal(err)
	}
	charts, err := readCharts(bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	got, err := toSample(charts)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("different result:\n got %+v\nwant %+v", got, want)
	}
	if len(charts) >= len(all)/2 {
		t.Fatalf("expected only the needed charts to be kept, got %d of %d", len(charts), len(all))
	}
}

func TestReadChartsRejectsGarbage(t *testing.T) {
	for _, bad := range []string{"", "[]", "not json", `{"system.cpu": `} {
		if _, err := readCharts(bytes.NewReader([]byte(bad))); err == nil {
			t.Errorf("%q should be an error", bad)
		}
	}
}
