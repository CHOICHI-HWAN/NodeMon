// SPDX-License-Identifier: GPL-3.0-or-later
package main

import (
	"encoding/json"
	"errors"
	"log"
	"os"
	"path/filepath"
	"time"
)

// config is what is saved after enrolment.
type config struct {
	Firebase *fbSaved `json:"firebase,omitempty"`
	// Usage is the running total of bytes sent to and received from Firebase since this computer was registered.
	Usage *usageSaved `json:"usage,omitempty"`
}

type usageSaved struct {
	Up   int64 `json:"up"`
	Down int64 `json:"down"`
}

func defaultConfigPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = "."
	}
	return filepath.Join(dir, "nodemon", "connector.json")
}

func loadConfig(path string) (config, error) {
	var c config
	b, err := os.ReadFile(path)
	if err != nil {
		return c, err
	}
	if err := json.Unmarshal(b, &c); err != nil {
		return c, err
	}
	if c.Firebase == nil {
		// Files written by builds that could also talk to a self-hosted NodeMon server (since removed) have no Firebase part.
		return c, errors.New("these saved credentials are not for Firebase (they were made by an older version); register the computer again with a new code")
	}
	return c, nil
}

func saveConfig(path string, c config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(c, "", "  ")
	return os.WriteFile(path, b, 0o600) // holds the device credential: owner-only
}

// credStore keeps the saved file in step with what changes while running: Firebase may rotate the
// refresh token, and the usage total keeps growing. It writes at most once a minute unless forced.
type credStore struct {
	path         string
	cfg          *config
	usage        *usageCounter
	savedRefresh string
	lastWrite    time.Time
	now          func() time.Time
}

func newCredStore(path string, cfg *config, usage *usageCounter) *credStore {
	s := &credStore{path: path, cfg: cfg, usage: usage, now: time.Now}
	s.savedRefresh = cfg.Firebase.RefreshToken
	s.lastWrite = s.now()
	return s
}

func (s *credStore) save(force bool) {
	up, down := s.usage.snapshot()
	rotated := s.cfg.Firebase.RefreshToken != s.savedRefresh
	grew := s.cfg.Usage == nil || up != s.cfg.Usage.Up || down != s.cfg.Usage.Down
	if !rotated && !(grew && (force || s.now().Sub(s.lastWrite) >= time.Minute)) {
		return
	}
	s.savedRefresh = s.cfg.Firebase.RefreshToken
	s.cfg.Usage = &usageSaved{Up: up, Down: down}
	s.lastWrite = s.now()
	if err := saveConfig(s.path, *s.cfg); err != nil {
		log.Printf("saving credentials: %v", err)
	}
}
