// SPDX-License-Identifier: GPL-3.0-or-later
//go:build linux

package main

import (
	"os"
	"strconv"
	"strings"
)

func hostUptime() (float64, bool) {
	b, err := os.ReadFile("/proc/uptime") // "12345.67 54321.00": seconds since boot, then idle time
	if err != nil {
		return 0, false
	}
	f := strings.Fields(string(b))
	if len(f) == 0 {
		return 0, false
	}
	v, err := strconv.ParseFloat(f[0], 64)
	return v, err == nil && v > 0
}
