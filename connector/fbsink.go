// SPDX-License-Identifier: GPL-3.0-or-later
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"
)

const (
	minuteKeepDays = 2   // minute buckets older than this are pruned
	hourKeepDays   = 100 // hour buckets older than this are pruned

	// maxQueued bounds the backlog kept while uploads fail: about a day of minute buckets.
	maxQueued  = 2000
	maxBackoff = 15 * time.Minute
)

// finished is one completed average, ready to be written at its database path.
type finished struct {
	path string
	rec  map[string]any
	hour bool
}

// bucketer turns a stream of samples into finished one-minute and one-hour averages. It does no I/O and has
// no clock of its own: the caller says what time it is, which keeps it easy to test.
type bucketer struct {
	uid       string
	minStart  time.Time
	min       agg
	hourStart time.Time
	hour      agg
}

// roll closes the minute (and hour) that ended before now. Call it before add for a sample taken at now.
func (b *bucketer) roll(now time.Time) []finished {
	var out []finished
	minute := now.Truncate(time.Minute)
	if b.min.n > 0 && !minute.Equal(b.minStart) {
		avg := b.min.mean()
		out = append(out, finished{
			path: fmt.Sprintf("m/%s/%s/%d", b.uid, b.minStart.Format("20060102"), b.minStart.Unix()),
			rec:  record(avg, map[string]any{"t": b.minStart.Unix()}),
		})
		b.hour.add(avg)
		if b.hourStart.IsZero() {
			b.hourStart = b.minStart.Truncate(time.Hour)
		}
		b.min = agg{}
		if !minute.Truncate(time.Hour).Equal(b.hourStart) {
			start := b.hourStart
			out = append(out, finished{
				path: fmt.Sprintf("h/%s/%s/%d", b.uid, start.Format("200601"), start.Unix()),
				rec:  record(b.hour.mean(), map[string]any{"t": start.Unix()}),
				hour: true,
			})
			b.hour, b.hourStart = agg{}, time.Time{}
		}
	}
	if b.min.n == 0 || !minute.Equal(b.minStart) {
		b.minStart = minute
	}
	return out
}

func (b *bucketer) add(s sample) { b.min.add(s) }

// backoff is the pause before the next upload attempt. After failures it doubles (up to 15 minutes), so a
// computer that was removed from the dashboard, or is offline, does not keep spending data on rejected requests.
type backoff struct {
	base  time.Duration
	fails int
}

func (b *backoff) wait() time.Duration {
	limit := max(maxBackoff, b.base)
	w := b.base
	for i := 0; i < b.fails && w < limit; i++ {
		w *= 2
	}
	return min(w, limit)
}

// fbSink measures often but uploads rarely: samples are averaged locally, and every uploadEvery
// the latest value and any finished one-minute / one-hour buckets go out in ONE request.
// Each request carries a long auth token, so fewer requests is what saves data.
type fbSink struct {
	f           *fbClient
	uploadEvery time.Duration
	bk          bucketer
	last        sample
	queue       map[string]any // finished buckets not yet uploaded, keyed by database path
	noBatch     bool           // set if the database refuses multi-path updates; we then write path by path
	usage       *usageCounter  // when set, running totals are uploaded with the latest value and hour buckets
	retry       backoff
	lastAttempt time.Time
	lastPruned  time.Time
}

func newFBSink(f *fbClient, uploadEvery time.Duration) *fbSink {
	return &fbSink{f: f, uploadEvery: uploadEvery, bk: bucketer{uid: f.cfg.UID}, queue: map[string]any{}, retry: backoff{base: uploadEvery}}
}

func (s *fbSink) Push(sm sample) error {
	now := s.f.now().UTC()
	for _, b := range s.bk.roll(now) {
		if b.hour {
			s.usageExtra(b.rec) // hour buckets also record the running totals, for the "last 24 hours" figure
		}
		s.queue[b.path] = b.rec
	}
	s.bk.add(sm)
	s.last = sm
	if !s.lastAttempt.IsZero() && now.Sub(s.lastAttempt) < s.retry.wait() {
		return nil
	}
	return s.flush(now)
}

// usageExtra adds the running upload/download totals (bytes) to a record when counting is on.
func (s *fbSink) usageExtra(extra map[string]any) map[string]any {
	if s.usage != nil {
		up, down := s.usage.snapshot()
		extra["up"], extra["down"] = up, down
	}
	return extra
}

// flush sends the queued buckets plus the latest value as a single multi-path update.
func (s *fbSink) flush(now time.Time) error {
	s.lastAttempt = now
	update := make(map[string]any, len(s.queue)+1)
	for k, v := range s.queue {
		update[k] = v
	}
	// ".sv" asks the database for its own clock, so online detection ignores this computer's clock.
	// "iv" tells the dashboard how often to expect an upload, so it can tell "quiet" from "offline".
	extra := map[string]any{"ts": map[string]string{".sv": "timestamp"}, "iv": int(s.uploadEvery.Seconds())}
	if s.last.Uptime > 0 {
		// The dashboard adds the time since this upload, so the value stays right between uploads.
		extra["ut"] = int64(s.last.Uptime)
	}
	update["latest/"+s.f.cfg.UID] = record(s.last, s.usageExtra(extra))
	if err := s.send(update); err != nil {
		s.retry.fails++
		s.trimQueue()
		return err
	}
	s.queue = map[string]any{}
	s.retry.fails = 0
	s.pruneIfDue(now)
	return nil
}

// trimQueue forgets the oldest minute buckets once the backlog is longer than we are willing to keep.
func (s *fbSink) trimQueue() {
	if len(s.queue) <= maxQueued {
		return
	}
	var minutes []string
	for k := range s.queue {
		if strings.HasPrefix(k, "m/") {
			minutes = append(minutes, k)
		}
	}
	sort.Strings(minutes) // the path ends in the day and the timestamp, so oldest sorts first within one computer
	for _, k := range minutes[:min(len(minutes), len(s.queue)-maxQueued)] {
		delete(s.queue, k)
	}
}

// send uploads the update in one request. If the database denies the multi-path form, it falls back
// to one write per path; if that is denied too, the device itself has lost access.
func (s *fbSink) send(update map[string]any) error {
	if !s.noBatch {
		_, err := s.f.db("PATCH", "", update, nil)
		if err == nil || !errors.Is(err, errDenied) {
			return err
		}
	}
	for path, v := range update {
		if _, err := s.f.db("PUT", path, v, nil); err != nil {
			return err
		}
		s.noBatch = true
	}
	return nil
}

// pruneIfDue deletes day/month buckets that are past retention. It runs at start-up and then daily.
func (s *fbSink) pruneIfDue(now time.Time) {
	if now.Sub(s.lastPruned) < 24*time.Hour {
		return
	}
	s.lastPruned = now
	prune := func(root string, keep time.Time, layout string) {
		raw, err := s.f.db("GET", root+"/"+s.f.cfg.UID, nil, url.Values{"shallow": {"true"}})
		if err != nil {
			return
		}
		var keys map[string]bool
		if json.Unmarshal(raw, &keys) != nil {
			return
		}
		cutoff := keep.Format(layout)
		for k := range keys {
			if k < cutoff {
				s.f.db("DELETE", root+"/"+s.f.cfg.UID+"/"+k, nil, nil)
			}
		}
	}
	prune("m", now.AddDate(0, 0, -minuteKeepDays), "20060102")
	prune("h", now.AddDate(0, 0, -hourKeepDays), "200601")
}
