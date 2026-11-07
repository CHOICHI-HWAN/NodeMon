// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"
)

// Netdata's /api/v1/allmetrics?format=json shape: chart id -> dimensions -> value.
type ndChart struct {
	Dimensions map[string]struct {
		Value *float64 `json:"value"`
	} `json:"dimensions"`
}

const (
	mib = 1024 * 1024
	gib = 1024 * mib
)

// maxNetdataBody bounds what we read from the agent; a busy host answers with a few MB.
const maxNetdataBody = 64 << 20

func fetchNetdata(c *http.Client, base string) (map[string]ndChart, error) {
	resp, err := c.Get(strings.TrimRight(base, "/") + "/api/v1/allmetrics?format=json")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("netdata returned %s", resp.Status)
	}
	return readCharts(io.LimitReader(resp.Body, maxNetdataBody))
}

// wantedChart reports whether toSample reads this chart. Everything else Netdata sends is skipped.
func wantedChart(id string) bool {
	switch id {
	case "system.cpu", "system.ram", "system.net", "system.load", "system.uptime", "apps.cpu", "apps.mem":
		return true
	}
	return strings.HasPrefix(id, "disk_space.")
}

// readCharts decodes only the charts we use. The agent describes every chart it has (often hundreds, with
// per-process and per-container detail), and decoding all of them into Go structures every ten seconds
// costs far more than the handful we need.
func readCharts(r io.Reader) (map[string]ndChart, error) {
	dec := json.NewDecoder(r)
	if t, err := dec.Token(); err != nil || t != json.Delim('{') {
		return nil, fmt.Errorf("netdata: unexpected response (not a JSON object): %v", err)
	}
	charts := map[string]ndChart{}
	for dec.More() {
		t, err := dec.Token()
		if err != nil {
			return nil, err
		}
		id, _ := t.(string)
		if wantedChart(id) {
			var c ndChart
			if err := dec.Decode(&c); err != nil {
				return nil, err
			}
			charts[id] = c
			continue
		}
		var skip json.RawMessage // consumed and dropped without building maps or structs
		if err := dec.Decode(&skip); err != nil {
			return nil, err
		}
	}
	return charts, nil
}

func (c ndChart) dim(name string) (float64, bool) {
	d, ok := c.Dimensions[name]
	if !ok || d.Value == nil || math.IsNaN(*d.Value) {
		return 0, false
	}
	return *d.Value, true
}

func (c ndChart) sum(names ...string) float64 {
	var t float64
	for _, n := range names {
		if v, ok := c.dim(n); ok {
			t += v
		}
	}
	return t
}

func (c ndChart) total() float64 {
	var t float64
	for name := range c.Dimensions {
		if v, ok := c.dim(name); ok {
			t += v
		}
	}
	return t
}

// toSample maps Netdata charts to our fixed schema. Chart/dimension names differ a
// little between Linux, macOS and Windows, so each metric falls back gracefully.
func toSample(charts map[string]ndChart) (sample, error) {
	var s sample
	cpu, ok := charts["system.cpu"]
	if !ok {
		return s, fmt.Errorf("system.cpu chart not found")
	}
	if idle, ok := cpu.dim("idle"); ok {
		s.CPU = math.Max(0, 100-idle)
	} else {
		s.CPU = cpu.total()
	}

	if ram, ok := charts["system.ram"]; ok {
		total := ram.total()
		var used float64
		if v, ok := ram.dim("used"); ok {
			used = v
		} else if v, ok := ram.dim("available"); ok {
			used = total - v
		} else {
			used = total - ram.sum("free", "cached", "buffers", "inactive", "purgeable", "speculative")
		}
		s.MemTotal, s.MemUsed = total*mib, math.Max(0, used)*mib
	}

	// Use the largest mounted filesystem as the "disk" figure.
	for id, ch := range charts {
		if !strings.HasPrefix(id, "disk_space.") {
			continue
		}
		used, total := ch.sum("used"), ch.sum("avail", "used", "reserved_for_root")
		if total*gib > s.DiskTotal {
			s.DiskTotal, s.DiskUsed = total*gib, used*gib
		}
	}

	if n, ok := charts["system.net"]; ok {
		const bytesPerKbit = 1000.0 / 8 // Netdata reports kilobits/s; sent is negative.
		rx, _ := n.dim("received")
		tx, _ := n.dim("sent")
		s.NetRx, s.NetTx = math.Abs(rx)*bytesPerKbit, math.Abs(tx)*bytesPerKbit
	}

	if l, ok := charts["system.load"]; ok {
		s.Load1, _ = l.dim("load1")
	}
	if u, ok := charts["system.uptime"]; ok {
		s.Uptime, _ = u.dim("uptime")
	}
	s.Apps = appsFrom(charts["apps.cpu"], charts["apps.mem"])
	return s, nil
}

// appsFrom reads Netdata's per-program charts (Linux and FreeBSD; elsewhere they do not exist and nothing is sent).
func appsFrom(cpu, mem ndChart) []appUse {
	use := map[string]appUse{}
	for name := range cpu.Dimensions {
		if v, ok := cpu.dim(name); ok {
			use[name] = appUse{Name: name, CPU: v}
		}
	}
	for name := range mem.Dimensions {
		if v, ok := mem.dim(name); ok {
			a := use[name]
			a.Name, a.Mem = name, v
			use[name] = a
		}
	}
	return pickApps(use)
}

func newHTTPClient() *http.Client { return &http.Client{Timeout: 10 * time.Second} }
