// SPDX-License-Identifier: GPL-3.0-or-later
package main

import (
	"regexp"
	"strconv"
	"time"
)

// withUptime fills in the computer's uptime (seconds since boot) when Netdata did not report it.
// Netdata's own value wins; the operating system is only asked as a fallback.
func withUptime(s *sample) {
	if s.Uptime > 0 {
		return
	}
	if u, ok := hostUptime(); ok {
		s.Uptime = u
	}
}

var boottimeSec = regexp.MustCompile(`sec\s*=\s*(\d+)`)

// parseBoottime reads the output of `sysctl -n kern.boottime` on macOS, e.g.
// "{ sec = 1730000000, usec = 123456 } Tue Oct 27 09:12:08 2026", and returns the uptime at `now`.
func parseBoottime(out string, now time.Time) (float64, bool) {
	m := boottimeSec.FindStringSubmatch(out)
	if m == nil {
		return 0, false
	}
	boot, err := strconv.ParseInt(m[1], 10, 64)
	if err != nil || boot <= 0 || boot > now.Unix() {
		return 0, false
	}
	return float64(now.Unix() - boot), true
}
