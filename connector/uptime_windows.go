// SPDX-License-Identifier: GPL-3.0-or-later
//go:build windows

package main

import "syscall"

var getTickCount64 = syscall.NewLazyDLL("kernel32.dll").NewProc("GetTickCount64")

// hostUptime returns the time since Windows started, which is what Task Manager shows as "Up time".
func hostUptime() (float64, bool) {
	ms, _, _ := getTickCount64.Call() // milliseconds since boot
	return float64(ms) / 1000, ms > 0
}
