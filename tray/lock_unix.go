// SPDX-License-Identifier: GPL-3.0-or-later
//go:build linux || darwin

package main

import (
	"os"
	"syscall"
)

var lockFile *os.File // kept open for the life of the process: the lock goes when the process does

// lockOnce reports whether this is the only copy running (it takes an exclusive lock on the file).
func lockOnce(path string) bool {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return true // cannot lock: better two icons than none
	}
	if syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB) != nil {
		f.Close()
		return false
	}
	lockFile = f
	return true
}
