// SPDX-License-Identifier: GPL-3.0-or-later
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// connectorStatus is what the connector tells the tray icon (nodemon-tray) through status.json, which sits next to
// the saved credentials. The connector runs unseen under systemd or launchd, so a file is the simplest way for a
// separate program to show how it is doing, and neither side has to know about the other.
type connectorStatus struct {
	Version    string `json:"version"`
	Updated    int64  `json:"updated"`     // unix seconds of this write; a tray that sees it stale knows the connector stopped
	LastSent   int64  `json:"last_sent"`   // unix seconds of the last upload that went through; 0 before the first one
	Problem    string `json:"problem"`     // empty while measuring and uploading work
	UploadSecs int    `json:"upload_secs"` // how often an upload is expected
}

// statusWriter writes status.json when something changed, and otherwise at most every refresh.
type statusWriter struct {
	path    string
	upload  time.Duration
	refresh time.Duration
	last    connectorStatus
	wrote   time.Time
	now     func() time.Time
}

func newStatusWriter(cfgPath string, upload time.Duration) *statusWriter {
	return &statusWriter{path: filepath.Join(filepath.Dir(cfgPath), "status.json"), upload: upload, refresh: 30 * time.Second, now: time.Now}
}

func (w *statusWriter) write(lastSent time.Time, problem string) {
	if w == nil {
		return
	}
	now := w.now()
	s := connectorStatus{Version: version, Updated: now.Unix(), Problem: problem, UploadSecs: int(w.upload.Seconds())}
	if !lastSent.IsZero() {
		s.LastSent = lastSent.Unix()
	}
	changed := s.LastSent != w.last.LastSent || s.Problem != w.last.Problem
	if !changed && now.Sub(w.wrote) < w.refresh {
		return
	}
	b, err := json.Marshal(s)
	if err != nil {
		return
	}
	tmp := w.path + ".tmp"
	if os.WriteFile(tmp, b, 0o644) != nil || os.Rename(tmp, w.path) != nil {
		os.Remove(tmp) // best effort: a status the icon cannot read must never stop the uploads
		return
	}
	w.last, w.wrote = s, now
}
