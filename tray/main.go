// SPDX-License-Identifier: GPL-3.0-or-later

// nodemon-tray puts a small icon next to the clock (Linux) or in the menu bar (macOS) that shows whether the
// NodeMon connector is running and uploading. It only reads the connector's status.json; it does not measure or
// upload anything itself, so it is safe to start and stop at any time.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"fyne.io/systray"
)

var version = "dev" // set at build time (-ldflags "-X main.version=...")

// statusFiles lists where the connector may have written status.json. By hand or as a macOS login item it uses the
// user's config folder; the systemd service the dashboard sets up on Linux keeps its files in /etc/nodemon.
func statusFiles() []string {
	var out []string
	if dir, err := os.UserConfigDir(); err == nil {
		out = append(out, filepath.Join(dir, "nodemon", "status.json"))
	}
	if runtime.GOOS == "linux" {
		out = append(out, "/etc/nodemon/status.json")
	}
	return out
}

// pickStatus reads the most recently refreshed of the candidate files, so the icon follows whichever connector is
// actually running even if an old copy of the file is left in the other place.
func pickStatus(paths []string) (*status, error) {
	var best *status
	var firstErr error
	for _, p := range paths {
		s, err := readStatus(p)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if best == nil || s.Updated > best.Updated {
			best = s
		}
	}
	if best == nil {
		if firstErr == nil {
			firstErr = os.ErrNotExist
		}
		return nil, firstErr
	}
	return best, nil
}

func openURL(u string) {
	cmd := "xdg-open"
	if runtime.GOOS == "darwin" {
		cmd = "open"
	}
	_ = exec.Command(cmd, u).Start()
}

func main() {
	statusPath := flag.String("status", "", "path of the connector's status.json (by default the usual places are tried)")
	dashboard := flag.String("dashboard", "", "dashboard address the menu opens")
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Printf("nodemon-tray %s (%s/%s)\n", version, runtime.GOOS, runtime.GOARCH)
		return
	}
	paths := statusFiles()
	if *statusPath != "" {
		paths = []string{*statusPath}
	}
	// A second copy would only add a second icon.
	if !lockOnce(filepath.Join(os.TempDir(), fmt.Sprintf("nodemon-tray-%d.lock", os.Getuid()))) {
		return
	}

	systray.Run(func() {
		systray.SetTitle("") // an empty title keeps macOS to the icon alone
		systray.SetTooltip("NodeMon")
		head := systray.AddMenuItem("NodeMon", "")
		head.Disable()
		detail := systray.AddMenuItem("", "")
		detail.Disable()
		systray.AddSeparator()
		if *dashboard != "" {
			open := systray.AddMenuItem("대시보드 열기", "")
			go func() {
				for range open.ClickedCh {
					openURL(*dashboard)
				}
			}()
		}
		quit := systray.AddMenuItem("아이콘 끄기", "이 아이콘만 끕니다. Connector는 계속 실행됩니다.")
		go func() {
			<-quit.ClickedCh
			systray.Quit()
		}()

		shown := state(-1)
		refresh := func() {
			s, err := pickStatus(paths)
			st, line1, line2 := evaluate(s, err, time.Now())
			head.SetTitle(line1)
			detail.SetTitle(line2)
			systray.SetTooltip("NodeMon: " + line1)
			if st != shown {
				systray.SetIcon(makeIcon(colors[st]))
				shown = st
			}
		}
		refresh()
		go func() {
			for range time.Tick(5 * time.Second) {
				refresh()
			}
		}()
	}, func() {})
}
