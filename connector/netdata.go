// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"sort"
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
	return strings.HasPrefix(id, "disk_space.") || strings.HasPrefix(id, "net.")
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

const bytesPerKbit = 1000.0 / 8 // Netdata reports kilobits/s; sent is negative.

// virtualNet names the interfaces that carry no traffic of their own (loopback, tunnels, bridges, containers, ...).
var virtualNet = []string{"lo", "utun", "awdl", "llw", "bridge", "gif", "stf", "ap", "anpi", "tun", "tap", "docker", "veth", "br-", "virbr", "vmnet", "vboxnet", "cali", "flannel", "cni"}

func physicalNet(id string) bool {
	name := strings.TrimPrefix(id, "net.")
	for _, p := range virtualNet {
		if strings.HasPrefix(name, p) {
			return false
		}
	}
	return name != ""
}

// netChartIDs names the charts that make up the computer's network traffic. Linux has one total (system.net);
// macOS has none, only one chart per interface, so the physical ones are added up.
func netChartIDs(charts map[string]ndChart) []string {
	if _, ok := charts["system.net"]; ok {
		return []string{"system.net"}
	}
	var ids []string
	for id := range charts {
		if strings.HasPrefix(id, "net.") && physicalNet(id) {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}

// fetchNetAverage asks Netdata for the average rate over the last `secs` seconds instead of the single last second,
// so traffic that comes in bursts between two measurements is not missed. ok is false if nothing could be read.
func fetchNetAverage(c *http.Client, base string, ids []string, secs int) (rx, tx float64, ok bool) {
	for _, id := range ids {
		u := fmt.Sprintf("%s/api/v1/data?chart=%s&after=-%d&points=1&group=average&format=json", strings.TrimRight(base, "/"), url.QueryEscape(id), secs)
		resp, err := c.Get(u)
		if err != nil {
			return 0, 0, false
		}
		r, t, good := parseNetAverage(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK || !good {
			return 0, 0, false
		}
		rx, tx, ok = rx+r, tx+t, true
	}
	return rx, tx, ok
}

// parseNetAverage reads {"labels":["time","received","sent"],"data":[[t, rx, tx]]} and returns bytes per second.
func parseNetAverage(r io.Reader) (rx, tx float64, ok bool) {
	var d struct {
		Labels []string     `json:"labels"`
		Data   [][]*float64 `json:"data"`
	}
	if json.NewDecoder(r).Decode(&d) != nil || len(d.Data) == 0 {
		return 0, 0, false
	}
	row := d.Data[len(d.Data)-1]
	for i, l := range d.Labels {
		if i >= len(row) || row[i] == nil || math.IsNaN(*row[i]) {
			continue
		}
		switch l {
		case "received":
			rx, ok = math.Abs(*row[i])*bytesPerKbit, true
		case "sent":
			tx, ok = math.Abs(*row[i])*bytesPerKbit, true
		}
	}
	return rx, tx, ok
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

	for _, id := range netChartIDs(charts) {
		n := charts[id]
		rx, _ := n.dim("received")
		tx, _ := n.dim("sent")
		s.NetRx += math.Abs(rx) * bytesPerKbit
		s.NetTx += math.Abs(tx) * bytesPerKbit
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
