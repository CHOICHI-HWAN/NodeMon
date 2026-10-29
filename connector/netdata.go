package main

import (
	"encoding/json"
	"fmt"
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

type sample struct {
	CPU       float64 `json:"cpu"`
	MemUsed   float64 `json:"mem_used"`
	MemTotal  float64 `json:"mem_total"`
	DiskUsed  float64 `json:"disk_used"`
	DiskTotal float64 `json:"disk_total"`
	NetRx     float64 `json:"net_rx"`
	NetTx     float64 `json:"net_tx"`
	Load1     float64 `json:"load1"`
}

const (
	mib = 1024 * 1024
	gib = 1024 * mib
)

func fetchNetdata(c *http.Client, base string) (map[string]ndChart, error) {
	resp, err := c.Get(strings.TrimRight(base, "/") + "/api/v1/allmetrics?format=json")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("netdata returned %s", resp.Status)
	}
	var charts map[string]ndChart
	if err := json.NewDecoder(resp.Body).Decode(&charts); err != nil {
		return nil, err
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
	return s, nil
}

func newHTTPClient() *http.Client { return &http.Client{Timeout: 10 * time.Second} }
