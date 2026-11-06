// SPDX-License-Identifier: GPL-3.0-or-later
//go:build darwin

package main

import (
	"os/exec"
	"time"
)

func hostUptime() (float64, bool) {
	out, err := exec.Command("sysctl", "-n", "kern.boottime").Output()
	if err != nil {
		return 0, false
	}
	return parseBoottime(string(out), time.Now())
}
