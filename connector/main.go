// SPDX-License-Identifier: GPL-3.0-or-later

// Command nodemon-connector reads a local Netdata Agent and sends the computer's resource usage to a
// NodeMon dashboard through Firebase Realtime Database.
package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
)

func main() {
	o, err := parseOptions(os.Args[1:], os.Stderr)
	switch {
	case errors.Is(err, flag.ErrHelp):
		return
	case errors.Is(err, errBadFlags):
		os.Exit(2) // the flag package already explained
	case err != nil:
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(2)
	}
	if o.showVersion {
		fmt.Printf("nodemon-connector %s (%s/%s, %s)\n", version, runtime.GOOS, runtime.GOARCH, runtime.Version())
		return
	}
	if o.tray {
		runTray(o) // measures in the background while the icon runs; returns only by ending the process
		return
	}
	r, err := newRunner(o)
	if err != nil {
		log.Fatal(err)
	}
	r.loop(o.interval) // measures until the process is stopped
}
