// SPDX-License-Identifier: GPL-3.0-or-later
//go:build !linux && !darwin && !windows

package main

// hostUptime is not implemented for other systems; the dashboard then simply shows no uptime.
func hostUptime() (float64, bool) { return 0, false }
