package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// config is saved after enrolment. Backend is "server" (NodeMon Go server) or "firebase".
type config struct {
	Backend  string   `json:"backend,omitempty"`
	Server   string   `json:"server,omitempty"`
	APIKey   string   `json:"api_key,omitempty"`
	HostID   int64    `json:"host_id,omitempty"`
	Firebase *fbSaved `json:"firebase,omitempty"`
}

func (c config) backend() string {
	if c.Backend == "" {
		return "server"
	}
	return c.Backend
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
	return c, json.Unmarshal(b, &c)
}

func saveConfig(path string, c config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(c, "", "  ")
	return os.WriteFile(path, b, 0o600)
}

// sink is where finished samples go.
type sink interface {
	Push(sample) error
}

var errRejected = errors.New("the server rejected this computer's credentials; it may have been removed. Re-enroll with a new code")

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
	io.Copy(io.Discard, resp.Body)
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
	body, _ := io.ReadAll(resp.Body)
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

func main() {
	hostname, _ := os.Hostname()
	var (
		server    = flag.String("server", "", "NodeMon server URL (server backend, with --code)")
		fbKey     = flag.String("firebase-api-key", "", "Firebase web API key (Firebase backend, with --code)")
		fbDB      = flag.String("firebase-db-url", "", "Realtime Database URL (Firebase backend, with --code)")
		fbAuthURL = flag.String("firebase-auth-url", "", "override the Firebase Auth endpoint (emulator only)")
		fbTokURL  = flag.String("firebase-token-url", "", "override the Firebase token endpoint (emulator only)")
		code      = flag.String("code", "", "one-time enrollment code from the dashboard")
		name      = flag.String("name", hostname, "display name for this computer")
		netdata   = flag.String("netdata", "http://127.0.0.1:19999", "local Netdata agent URL")
		interval  = flag.Duration("interval", 10*time.Second, "how often to measure (server backend: also how often to send)")
		upload    = flag.Duration("upload-every", time.Minute, "Firebase backend: how often to upload; measurements in between are averaged locally")
		cfgPath   = flag.String("config", defaultConfigPath(), "path of the saved credentials")
	)
	flag.Parse()
	client := newHTTPClient()

	cfg, loadErr := loadConfig(*cfgPath)
	if *code != "" {
		var err error
		switch {
		case *fbDB != "":
			if *fbKey == "" {
				log.Fatal("--firebase-api-key is required together with --firebase-db-url")
			}
			var saved *fbSaved
			saved, err = enrollFirebase(client, fbSaved{APIKey: *fbKey, DatabaseURL: *fbDB, AuthURL: *fbAuthURL, TokenURL: *fbTokURL}, *code, *name, runtime.GOOS)
			cfg = config{Backend: "firebase", Firebase: saved}
		case *server != "":
			cfg, err = enrollServer(client, strings.TrimRight(*server, "/"), *code, *name)
		default:
			log.Fatal("--code needs either --server or --firebase-db-url")
		}
		if err != nil {
			log.Fatal(err)
		}
		if err := saveConfig(*cfgPath, cfg); err != nil {
			log.Fatalf("saving credentials: %v", err)
		}
		log.Printf("enrolled; credentials saved to %s", *cfgPath)
	} else if loadErr != nil {
		log.Fatalf("no credentials at %s (run once with --code): %v", *cfgPath, loadErr)
	}

	var out sink
	var persist func()
	switch cfg.backend() {
	case "firebase":
		fc := newFBClient(client, cfg.Firebase)
		out = newFBSink(fc, *upload)
		// Firebase may rotate the refresh token; keep the saved copy current.
		last := cfg.Firebase.RefreshToken
		persist = func() {
			if cfg.Firebase.RefreshToken != last {
				last = cfg.Firebase.RefreshToken
				if err := saveConfig(*cfgPath, cfg); err != nil {
					log.Printf("saving refreshed credentials: %v", err)
				}
			}
		}
	default:
		out = httpSink{c: client, server: cfg.Server, key: cfg.APIKey}
	}

	log.Printf("measuring %s every %s, backend %s", *netdata, *interval, cfg.backend())
	for ; ; time.Sleep(*interval) {
		charts, err := fetchNetdata(client, *netdata)
		if err != nil {
			log.Printf("netdata: %v", err)
			continue
		}
		s, err := toSample(charts)
		if err != nil {
			log.Printf("netdata: %v", err)
			continue
		}
		if err := out.Push(s); err != nil {
			if errors.Is(err, errDenied) {
				err = errRejected
			}
			log.Printf("send: %v", err)
		}
		if persist != nil {
			persist()
		}
	}
}
