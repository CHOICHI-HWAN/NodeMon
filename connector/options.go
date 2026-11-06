// SPDX-License-Identifier: GPL-3.0-or-later
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"time"
)

// version is set at build time: go build -ldflags "-X main.version=v1.2.3".
var version = "dev"

// errBadFlags means the flag package has already told the user what was wrong.
var errBadFlags = errors.New("invalid command line")

type options struct {
	fbKey       string
	fbDB        string
	fbAuthURL   string
	fbTokenURL  string
	code        string
	name        string
	netdata     string
	cfgPath     string
	interval    time.Duration
	upload      time.Duration
	compress    bool
	showVersion bool
}

func parseOptions(args []string, errOut io.Writer) (options, error) {
	hostname, _ := os.Hostname()
	o := options{}
	fs := flag.NewFlagSet("nodemon-connector", flag.ContinueOnError)
	fs.SetOutput(errOut)
	fs.StringVar(&o.fbKey, "firebase-api-key", "", "Firebase web API key (with --code)")
	fs.StringVar(&o.fbDB, "firebase-db-url", "", "Realtime Database URL (with --code)")
	fs.StringVar(&o.fbAuthURL, "firebase-auth-url", "", "override the Firebase Auth endpoint (emulator only)")
	fs.StringVar(&o.fbTokenURL, "firebase-token-url", "", "override the Firebase token endpoint (emulator only)")
	fs.StringVar(&o.code, "code", "", "one-time enrollment code from the dashboard")
	fs.StringVar(&o.name, "name", hostname, "display name for this computer")
	fs.StringVar(&o.netdata, "netdata", "http://127.0.0.1:19999", "local Netdata agent URL")
	fs.DurationVar(&o.interval, "interval", 10*time.Second, "how often to measure")
	fs.BoolVar(&o.compress, "compress", false, "experimental: gzip upload bodies; falls back to plain if the database refuses them")
	fs.DurationVar(&o.upload, "upload-every", 3*time.Minute, "how often to upload; measurements in between are averaged locally")
	fs.StringVar(&o.cfgPath, "config", defaultConfigPath(), "path of the saved credentials")
	fs.BoolVar(&o.showVersion, "version", false, "print the version and exit")
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "NodeMon connector: sends this computer's resource usage, read from a local Netdata Agent, to your NodeMon dashboard.\n"+
			"NodeMon is an independent project, not affiliated with Netdata, Inc. Netdata is a trademark of Netdata, Inc.\n\nUsage:\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return o, err
		}
		return o, errBadFlags
	}
	if o.showVersion {
		return o, nil
	}
	return o, o.validate()
}

func (o options) validate() error {
	// A pause below one second would keep Netdata and this computer busy for no benefit.
	if o.interval < time.Second {
		return fmt.Errorf("--interval must be at least 1s (got %s)", o.interval)
	}
	// The database rules accept an upload interval of up to a day.
	if o.upload < time.Second || o.upload > 24*time.Hour {
		return fmt.Errorf("--upload-every must be between 1s and 24h (got %s)", o.upload)
	}
	if o.code == "" {
		return nil
	}
	if o.fbDB == "" || o.fbKey == "" {
		return errors.New("--code needs --firebase-db-url and --firebase-api-key (the dashboard's add-computer window shows the full command)")
	}
	for flagName, raw := range map[string]string{"--firebase-db-url": o.fbDB, "--firebase-auth-url": o.fbAuthURL, "--firebase-token-url": o.fbTokenURL} {
		if raw == "" {
			continue
		}
		if err := checkSecureURL(raw); err != nil {
			return fmt.Errorf("%s: %w", flagName, err)
		}
	}
	return nil
}

// checkSecureURL accepts https, and plain http only for the local emulator. The database address receives
// a login token with every request, so sending it over plain http to another machine would expose it.
func checkSecureURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return fmt.Errorf("%q is not a valid address", raw)
	}
	switch {
	case u.Scheme == "https":
		return nil
	case u.Scheme == "http" && isLoopback(u.Hostname()):
		return nil
	}
	return fmt.Errorf("%q must start with https:// (a login token is sent with every request; plain http is only allowed for localhost)", raw)
}

func isLoopback(host string) bool {
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}
