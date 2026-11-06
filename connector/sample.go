// SPDX-License-Identifier: GPL-3.0-or-later
package main

import "math"

// sample is one measurement of the computer, whichever backend it is sent to.
type sample struct {
	CPU       float64 `json:"cpu"`
	MemUsed   float64 `json:"mem_used"`
	MemTotal  float64 `json:"mem_total"`
	DiskUsed  float64 `json:"disk_used"`
	DiskTotal float64 `json:"disk_total"`
	NetRx     float64 `json:"net_rx"`
	NetTx     float64 `json:"net_tx"`
	Load1     float64 `json:"load1"`
	Uptime    float64 `json:"uptime"` // seconds since the computer booted; not averaged, only the latest is sent
}

// agg accumulates samples and reports their mean.
type agg struct {
	n   int
	sum sample
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
}

// mean is the average of what was added. Uptime is deliberately left out: it is a point-in-time value.
func (a *agg) mean() sample {
	d := float64(a.n)
	return sample{
		CPU: a.sum.CPU / d, MemUsed: a.sum.MemUsed / d, MemTotal: a.sum.MemTotal / d,
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
	for k, v := range extra {
		m[k] = v
	}
	return m
}
