// SPDX-License-Identifier: GPL-3.0-or-later
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
)

// sink is where finished samples go.
type sink interface {
	Push(sample) error
}

var errRejected = errors.New("the server rejected this computer's credentials; it may have been removed. Re-enroll with a new code")

// The code below is the client side of the self-hosted NodeMon server (server/), the alternative to Firebase.

func postJSON(c *http.Client, url, key string, body any) (*http.Response, error) {
	b, _ := json.Marshal(body)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	return c.Do(req)
}

type httpSink struct {
	c      *http.Client
	server string
	key    string
}

func (h httpSink) Push(s sample) error {
	resp, err := postJSON(h.c, h.server+"/api/agent/ingest", h.key, s)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, io.LimitReader(resp.Body, maxReply))
	switch {
	case resp.StatusCode == http.StatusUnauthorized:
		return errRejected
	case resp.StatusCode >= 300:
		return fmt.Errorf("server returned %s", resp.Status)
	}
	return nil
}

func enrollServer(c *http.Client, server, code, name string) (config, error) {
	resp, err := postJSON(c, server+"/api/agent/enroll", "", map[string]string{
		"code": code, "name": name, "os": runtime.GOOS,
	})
	if err != nil {
		return config{}, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxReply))
	if resp.StatusCode != http.StatusOK {
		return config{}, fmt.Errorf("enroll failed: %s %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var out struct {
		HostID int64  `json:"host_id"`
		APIKey string `json:"api_key"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return config{}, err
	}
	return config{Backend: "server", Server: server, APIKey: out.APIKey, HostID: out.HostID}, nil
}
