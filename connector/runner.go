// SPDX-License-Identifier: GPL-3.0-or-later
package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
)

// quietLog prints a recurring problem at most once per interval. Without it, a stopped Netdata would write the
// same line every few seconds for as long as it stays down. It also says so once the problem is gone.
type quietLog struct {
	every   time.Duration
	last    map[string]time.Time
	failing map[string]bool
	now     func() time.Time
}

func newQuietLog(every time.Duration) *quietLog {
	return &quietLog{every: every, last: map[string]time.Time{}, failing: map[string]bool{}, now: time.Now}
}

func (q *quietLog) problem(key, format string, args ...any) {
	q.failing[key] = true
	if t, ok := q.last[key]; ok && q.now().Sub(t) < q.every {
		return
	}
	q.last[key] = q.now()
	log.Printf(format, args...)
}

func (q *quietLog) recovered(key, message string) {
	if q.failing[key] {
		delete(q.failing, key)
		delete(q.last, key)
		log.Print(message)
	}
}

// runner repeats measure-and-send until the process is stopped.
type runner struct {
	mu      sync.Mutex // held while a measurement/upload is in flight, so a file is never written mid-request
	client  *http.Client
	netdata string
	out     sink
	store   *credStore // nil when the backend keeps nothing locally
	logs    *quietLog
}

// tick takes one measurement, hands it to the sink and keeps the saved credentials current.
func (r *runner) tick() {
	charts, err := fetchNetdata(r.client, r.netdata)
	if err != nil {
		r.logs.problem("netdata", "netdata: %v", err)
		return
	}
	s, err := toSample(charts)
	if err != nil {
		r.logs.problem("netdata", "netdata: %v", err)
		return
	}
	r.logs.recovered("netdata", "netdata: readings are coming through again")
	withUptime(&s)
	if err := r.out.Push(s); err != nil {
		if errors.Is(err, errDenied) {
			err = errRejected
		}
		r.logs.problem("send", "send: %v", err)
	}
	if r.store != nil {
		r.store.save(false)
	}
}

// onStop saves the running usage total when the process is asked to stop (Ctrl+C, service stop),
// so a restart does not lose it. A second signal ends the process at once.
func (r *runner) onStop() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigs
		signal.Reset(os.Interrupt, syscall.SIGTERM)
		r.mu.Lock()
		if r.store != nil {
			r.store.save(true)
		}
		os.Exit(0)
	}()
}

func (r *runner) loop(interval time.Duration) {
	r.onStop()
	for {
		r.mu.Lock()
		r.tick()
		r.mu.Unlock()
		time.Sleep(interval)
	}
}

// obtainConfig enrolls this computer when a code was given, otherwise loads what an earlier run saved.
func obtainConfig(o options) (config, *usageCounter, error) {
	if o.code == "" {
		cfg, err := loadConfig(o.cfgPath)
		if err != nil {
			return cfg, nil, fmt.Errorf("no credentials at %s (run once with --code): %w", o.cfgPath, err)
		}
		usage := newUsageCounter(0, 0)
		if cfg.Usage != nil {
			usage = newUsageCounter(cfg.Usage.Up, cfg.Usage.Down) // carry the total across restarts
		}
		return cfg, usage, nil
	}

	usage := newUsageCounter(0, 0)
	var cfg config
	var err error
	if o.fbDB != "" {
		var saved *fbSaved
		saved, err = enrollFirebase(newCountingClient(usage, 5*time.Minute),
			fbSaved{APIKey: o.fbKey, DatabaseURL: o.fbDB, AuthURL: o.fbAuthURL, TokenURL: o.fbTokenURL}, o.code, o.name, runtime.GOOS)
		up, down := usage.snapshot()
		cfg = config{Backend: "firebase", Firebase: saved, Usage: &usageSaved{Up: up, Down: down}}
	} else {
		cfg, err = enrollServer(newHTTPClient(), strings.TrimRight(o.server, "/"), o.code, o.name)
	}
	if err != nil {
		return cfg, nil, err
	}
	if err := saveConfig(o.cfgPath, cfg); err != nil {
		return cfg, nil, fmt.Errorf("saving credentials: %w", err)
	}
	log.Printf("enrolled; credentials saved to %s", o.cfgPath)
	return cfg, usage, nil
}

// newBackend builds the sink for the configured backend, and the store that keeps its state saved.
func newBackend(o options, cfg *config, usage *usageCounter, local *http.Client) (sink, *credStore) {
	if cfg.backend() != "firebase" {
		return httpSink{c: local, server: cfg.Server, key: cfg.APIKey}, nil
	}
	// Keep the connection open between uploads (at least 5 minutes, or twice the upload interval).
	idle := max(2*o.upload, 5*time.Minute)
	fc := newFBClient(newCountingClient(usage, idle), cfg.Firebase)
	fc.compress = o.compress
	fs := newFBSink(fc, o.upload)
	fs.usage = usage
	return fs, newCredStore(o.cfgPath, cfg, usage)
}

// newRunner enrolls or loads the credentials and wires the backend; the caller then runs its loop.
func newRunner(o options) (*runner, error) {
	cfg, usage, err := obtainConfig(o)
	if err != nil {
		return nil, err
	}
	local := newHTTPClient() // for the local Netdata agent: never counted as upload
	out, store := newBackend(o, &cfg, usage, local)
	log.Printf("measuring %s every %s, backend %s", o.netdata, o.interval, cfg.backend())
	return &runner{client: local, netdata: o.netdata, out: out, store: store, logs: newQuietLog(10 * time.Minute)}, nil
}
