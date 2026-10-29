package main

import (
	"bytes"
	"encoding/json"
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

type config struct {
	Server string `json:"server"`
	APIKey string `json:"api_key"`
	HostID int64  `json:"host_id"`
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

func enroll(c *http.Client, server, code, name string) (config, error) {
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
	return config{Server: server, APIKey: out.APIKey, HostID: out.HostID}, nil
}

func main() {
	hostname, _ := os.Hostname()
	var (
		server   = flag.String("server", "", "NodeMon server URL (required for --code)")
		code     = flag.String("code", "", "one-time enrollment code from the dashboard")
		name     = flag.String("name", hostname, "display name for this computer")
		netdata  = flag.String("netdata", "http://127.0.0.1:19999", "local Netdata agent URL")
		interval = flag.Duration("interval", 10*time.Second, "send interval")
		cfgPath  = flag.String("config", defaultConfigPath(), "path of the saved credentials")
	)
	flag.Parse()
	client := newHTTPClient()

	cfg, err := loadConfig(*cfgPath)
	if *code != "" {
		if *server == "" {
			log.Fatal("--server is required together with --code")
		}
		cfg, err = enroll(client, strings.TrimRight(*server, "/"), *code, *name)
		if err != nil {
			log.Fatal(err)
		}
		if err := saveConfig(*cfgPath, cfg); err != nil {
			log.Fatalf("saving credentials: %v", err)
		}
		log.Printf("enrolled as host #%d; credentials saved to %s", cfg.HostID, *cfgPath)
	} else if err != nil {
		log.Fatalf("no credentials at %s (run once with --server and --code): %v", *cfgPath, err)
	}

	log.Printf("sending metrics from %s to %s every %s", *netdata, cfg.Server, *interval)
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
		resp, err := postJSON(client, cfg.Server+"/api/agent/ingest", cfg.APIKey, s)
		if err != nil {
			log.Printf("send: %v", err)
			continue
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		switch {
		case resp.StatusCode == http.StatusUnauthorized:
			log.Printf("server rejected the key; the computer may have been removed. Re-enroll with a new code.")
		case resp.StatusCode >= 300:
			log.Printf("send: server returned %s", resp.Status)
		}
	}
}
