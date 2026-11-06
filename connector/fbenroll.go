// SPDX-License-Identifier: GPL-3.0-or-later
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

func normalizeCode(code string) string {
	return strings.ToUpper(strings.NewReplacer("-", "", " ", "").Replace(code))
}

// enrollFirebase pairs this computer with the dashboard owner who issued the code.
func enrollFirebase(c *http.Client, cfg fbSaved, code, name, osName string) (*fbSaved, error) {
	f := newFBClient(c, &cfg)
	code = normalizeCode(code)
	if err := f.signUp(); err != nil {
		return nil, err
	}
	raw, err := f.db("GET", "pairings/"+code, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("reading pairing code: %w", err)
	}
	var pairing struct {
		Owner string `json:"owner"`
		Exp   int64  `json:"exp"`
	}
	if json.Unmarshal(raw, &pairing) != nil || pairing.Owner == "" {
		return nil, errors.New("invalid or expired code")
	}
	if _, err := f.db("PUT", "pairings/"+code+"/claimedBy", f.cfg.UID, nil); err != nil {
		return nil, fmt.Errorf("claiming code (already used or expired?): %w", err)
	}
	created := f.now().UnixMilli()
	dev := map[string]any{"owner": pairing.Owner, "code": code, "name": name, "os": osName, "createdAt": created}
	if _, err := f.db("PUT", "devices/"+f.cfg.UID, dev, nil); err != nil {
		return nil, fmt.Errorf("registering device: %w", err)
	}
	idx := map[string]any{"name": name, "os": osName, "createdAt": created}
	if _, err := f.db("PUT", "users/"+pairing.Owner+"/devices/"+f.cfg.UID, idx, nil); err != nil {
		return nil, fmt.Errorf("adding to dashboard: %w", err)
	}
	f.cfg.Owner = pairing.Owner
	return f.cfg, nil
}
