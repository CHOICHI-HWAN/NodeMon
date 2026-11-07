// SPDX-License-Identifier: GPL-3.0-or-later
package main

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode/utf8"
)

// sample is one measurement of the computer, whichever backend it is sent to.
type sample struct {
	CPU       float64  `json:"cpu"`
	MemUsed   float64  `json:"mem_used"`
	MemTotal  float64  `json:"mem_total"`
	DiskUsed  float64  `json:"disk_used"`
	DiskTotal float64  `json:"disk_total"`
	NetRx     float64  `json:"net_rx"`
	NetTx     float64  `json:"net_tx"`
	Load1     float64  `json:"load1"`
	Uptime    float64  `json:"uptime"` // seconds since the computer booted; not averaged, only the latest is sent
	Apps      []appUse `json:"-"`      // the programs that used the most CPU or memory; empty where Netdata has no per-program data
}

// appUse is what one program (Netdata's "apps" group, e.g. nginx or chrome) used. CPU is a percentage of ONE core,
// as in `top`, so a program that keeps two cores busy shows 200. Mem is the resident memory in MiB.
type appUse struct {
	Name string
	CPU  float64
	Mem  float64
}

const (
	appsByCPU = 4  // how many programs are kept for their CPU use...
	appsByMem = 3  // ...plus the biggest memory users, if not among them already
	appName   = 30 // longest name kept, in characters
)

// pickApps keeps the biggest CPU users and the biggest memory users. Sending every program would cost far more
// data than it is worth: the dashboard only needs the ones that matter.
func pickApps(use map[string]appUse) []appUse {
	all := make([]appUse, 0, len(use))
	for _, a := range use {
		all = append(all, a)
	}
	sort.Slice(all, func(i, j int) bool {
		return all[i].CPU > all[j].CPU || all[i].CPU == all[j].CPU && all[i].Name < all[j].Name
	})
	var out []appUse
	chosen := map[string]bool{}
	for _, a := range all {
		if len(out) == appsByCPU || a.CPU < 0.1 {
			break
		}
		out, chosen[a.Name] = append(out, a), true
	}
	sort.Slice(all, func(i, j int) bool {
		return all[i].Mem > all[j].Mem || all[i].Mem == all[j].Mem && all[i].Name < all[j].Name
	})
	for i, a := range all { // the three biggest memory users, unless they are already in
		if i == appsByMem || a.Mem < 1 {
			break
		}
		if !chosen[a.Name] {
			out = append(out, a)
		}
	}
	return out
}

// cleanAppName makes a program name safe for the compact text form below.
func cleanAppName(n string) string {
	n = strings.Map(func(r rune) rune {
		if r == '|' || r == ':' || r < 0x20 || r == 0x7f {
			return '_'
		}
		return r
	}, n)
	if utf8.RuneCountInString(n) > appName {
		n = string([]rune(n)[:appName])
	}
	return n
}

// encodeApps writes the programs as one short text, "name:cpu:mem|name:cpu:mem". One string costs about half of
// what the same data costs as a list of objects, because the field names are not repeated for every program.
func encodeApps(apps []appUse) string {
	parts := make([]string, 0, len(apps))
	for _, a := range apps {
		parts = append(parts, fmt.Sprintf("%s:%g:%g", cleanAppName(a.Name), round(a.CPU, 1), round(a.Mem, 0)))
	}
	return strings.Join(parts, "|")
}

// agg accumulates samples and reports their mean.
type agg struct {
	n    int
	sum  sample
	apps map[string]appUse // running sums per program; a program missing from a sample counts as zero for it
}

func (a *agg) add(s sample) {
	a.n++
	a.sum.CPU += s.CPU
	a.sum.MemUsed += s.MemUsed
	a.sum.MemTotal += s.MemTotal
	a.sum.DiskUsed += s.DiskUsed
	a.sum.DiskTotal += s.DiskTotal
	a.sum.NetRx += s.NetRx
	a.sum.NetTx += s.NetTx
	a.sum.Load1 += s.Load1
	for _, p := range s.Apps {
		if a.apps == nil {
			a.apps = map[string]appUse{}
		}
		t := a.apps[p.Name]
		a.apps[p.Name] = appUse{Name: p.Name, CPU: t.CPU + p.CPU, Mem: t.Mem + p.Mem}
	}
}

// mean is the average of what was added. Uptime is deliberately left out: it is a point-in-time value.
func (a *agg) mean() sample {
	d := float64(a.n)
	mean := map[string]appUse{}
	for n, t := range a.apps {
		mean[n] = appUse{Name: n, CPU: t.CPU / d, Mem: t.Mem / d}
	}
	return sample{
		Apps: pickApps(mean),
		CPU:  a.sum.CPU / d, MemUsed: a.sum.MemUsed / d, MemTotal: a.sum.MemTotal / d,
		DiskUsed: a.sum.DiskUsed / d, DiskTotal: a.sum.DiskTotal / d,
		NetRx: a.sum.NetRx / d, NetTx: a.sum.NetTx / d, Load1: a.sum.Load1 / d,
	}
}

func round(v float64, places int) float64 {
	p := math.Pow10(places)
	return math.Round(v*p) / p
}

// record is the stored shape of a sample; extra holds the time field ("t" for averages, "ts" for latest).
// Values are rounded because averages otherwise print as 17-digit floats, which would double the upload size.
func record(s sample, extra map[string]any) map[string]any {
	m := map[string]any{
		"cpu": round(s.CPU, 2), "load1": round(s.Load1, 2),
		"mem_used": round(s.MemUsed, 0), "mem_total": round(s.MemTotal, 0),
		"disk_used": round(s.DiskUsed, 0), "disk_total": round(s.DiskTotal, 0),
		"net_rx": round(s.NetRx, 0), "net_tx": round(s.NetTx, 0),
	}
	if len(s.Apps) > 0 {
		m["ap"] = encodeApps(s.Apps)
	}
	for k, v := range extra {
		m[k] = v
	}
	return m
}
