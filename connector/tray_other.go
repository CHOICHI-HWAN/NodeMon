// SPDX-License-Identifier: GPL-3.0-or-later
//go:build !windows

package main

// trayAvailable is false here: the tray icon is Windows only (systemd and launchd already run the connector unseen).
const trayAvailable = false

func runTray(options) {}
