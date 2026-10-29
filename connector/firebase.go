package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultAuthURL  = "https://identitytoolkit.googleapis.com"
	defaultTokenURL = "https://securetoken.googleapis.com"
	minuteKeepDays  = 2   // minute buckets older than this are pruned
	hourKeepDays    = 100 // hour buckets older than this are pruned
)

var errDenied = errors.New("access denied by Firebase rules")

// fbSaved is what we persist after enrolment. The refresh token is the device credential.
type fbSaved struct {
	APIKey       string `json:"api_key"`
	DatabaseURL  string `json:"database_url"`
	AuthURL      string `json:"auth_url,omitempty"`  // overridden only for the local emulator
	TokenURL     string `json:"token_url,omitempty"` // overridden only for the local emulator
	UID          string `json:"uid"`
	Owner        string `json:"owner"`
	RefreshToken string `json:"refresh_token"`
}

type fbClient struct {
	cfg     *fbSaved
	http    *http.Client
	idToken string
	expiry  time.Time
	now     func() time.Time
}

func newFBClient(c *http.Client, cfg *fbSaved) *fbClient {
	if cfg.AuthURL == "" {
		cfg.AuthURL = defaultAuthURL
	}
	if cfg.TokenURL == "" {
		cfg.TokenURL = defaultTokenURL
	}
	return &fbClient{cfg: cfg, http: c, now: time.Now}
}

func (f *fbClient) postAuth(endpoint string, body io.Reader, contentType string, out any) error {
	resp, err := f.http.Post(endpoint, contentType, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("firebase auth: %s %s", resp.Status, strings.TrimSpace(string(b)))
	}
	return json.Unmarshal(b, out)
}

// signUp creates the anonymous account that identifies this computer.
func (f *fbClient) signUp() error {
	var r struct {
		IDToken      string `json:"idToken"`
		RefreshToken string `json:"refreshToken"`
		ExpiresIn    string `json:"expiresIn"`
		LocalID      string `json:"localId"`
	}
	ep := fmt.Sprintf("%s/v1/accounts:signUp?key=%s", f.cfg.AuthURL, url.QueryEscape(f.cfg.APIKey))
	if err := f.postAuth(ep, strings.NewReader(`{"returnSecureToken":true}`), "application/json", &r); err != nil {
		return err
	}
	f.setToken(r.IDToken, r.ExpiresIn)
	f.cfg.UID, f.cfg.RefreshToken = r.LocalID, r.RefreshToken
	return nil
}

func (f *fbClient) setToken(id, expiresIn string) {
	var secs int
	fmt.Sscanf(expiresIn, "%d", &secs)
	if secs == 0 {
		secs = 3600
	}
	f.idToken, f.expiry = id, f.now().Add(time.Duration(secs)*time.Second)
}

func (f *fbClient) refresh() error {
	var r struct {
		IDToken      string `json:"id_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    string `json:"expires_in"`
	}
	form := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {f.cfg.RefreshToken}}
	ep := fmt.Sprintf("%s/v1/token?key=%s", f.cfg.TokenURL, url.QueryEscape(f.cfg.APIKey))
	if err := f.postAuth(ep, strings.NewReader(form.Encode()), "application/x-www-form-urlencoded", &r); err != nil {
		return err
	}
	f.setToken(r.IDToken, r.ExpiresIn)
	if r.RefreshToken != "" {
		f.cfg.RefreshToken = r.RefreshToken
	}
	return nil
}

func (f *fbClient) token() (string, error) {
	if f.idToken == "" || f.now().After(f.expiry.Add(-5*time.Minute)) {
		if err := f.refresh(); err != nil {
			return "", err
		}
	}
	return f.idToken, nil
}

func (f *fbClient) dbURL(path, token string, extra url.Values) (string, error) {
	u, err := url.Parse(f.cfg.DatabaseURL)
	if err != nil {
		return "", err
	}
	q := u.Query() // keeps ?ns=... when pointing at the emulator
	for k, v := range extra {
		q[k] = v
	}
	q.Set("auth", token)
	u.Path = "/" + strings.Trim(path, "/") + ".json"
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// db performs one REST call; a 401 triggers a single token refresh and retry.
func (f *fbClient) db(method, path string, body any, extra url.Values) ([]byte, error) {
	for attempt := 0; ; attempt++ {
		tok, err := f.token()
		if err != nil {
			return nil, err
		}
		u, err := f.dbURL(path, tok, extra)
		if err != nil {
			return nil, err
		}
		var rd io.Reader
		if body != nil {
			b, _ := json.Marshal(body)
			rd = bytes.NewReader(b)
		}
		req, _ := http.NewRequest(method, u, rd)
		resp, err := f.http.Do(req)
		if err != nil {
			return nil, err
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		switch {
		case resp.StatusCode == http.StatusOK:
			return b, nil
		case resp.StatusCode == http.StatusUnauthorized && attempt == 0:
			f.idToken = "" // force a fresh token, then retry once
		case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
			return nil, errDenied
		default:
			return nil, fmt.Errorf("database %s %s: %s", method, path, resp.Status)
		}
	}
}

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

// agg accumulates samples and reports their mean.
type agg struct {
	n   int
	sum sample
}

func (a *agg) add(s sample) {
	a.n++
	a.sum.CPU += s.CPU
	a.sum.MemUsed += s.MemUsed
	a.sum.MemTotal += s.MemTotal
	a.sum.DiskUsed += s.DiskUsed
	a.sum.DiskTotal += s.DiskTotal
	a.sum.NetRx += s.NetRx
	a.sum.NetTx += s.NetTx
	a.sum.Load1 += s.Load1
}

func (a *agg) mean() sample {
	d := float64(a.n)
	return sample{a.sum.CPU / d, a.sum.MemUsed / d, a.sum.MemTotal / d, a.sum.DiskUsed / d, a.sum.DiskTotal / d, a.sum.NetRx / d, a.sum.NetTx / d, a.sum.Load1 / d}
}

// record is the stored shape of a sample; extra holds the time field ("t" for averages, "ts" for latest).
func record(s sample, extra map[string]any) map[string]any {
	m := map[string]any{
		"cpu": s.CPU, "mem_used": s.MemUsed, "mem_total": s.MemTotal,
		"disk_used": s.DiskUsed, "disk_total": s.DiskTotal, "net_rx": s.NetRx, "net_tx": s.NetTx, "load1": s.Load1,
	}
	for k, v := range extra {
		m[k] = v
	}
	return m
}

// fbSink writes the live value every push, plus one-minute and one-hour averages.
// Averages keep long-range charts cheap to download on the free plan.
type fbSink struct {
	f          *fbClient
	minStart   time.Time
	min        agg
	hourStart  time.Time
	hour       agg
	lastPruned time.Time
}

func newFBSink(f *fbClient) *fbSink { return &fbSink{f: f} }

func (s *fbSink) Push(sm sample) error {
	now := s.f.now().UTC()
	if err := s.rollover(now); err != nil {
		return err
	}
	s.min.add(sm)
	// ".sv" asks the database for its own clock, so online detection ignores this computer's clock.
	body := record(sm, map[string]any{"ts": map[string]string{".sv": "timestamp"}})
	_, err := s.f.db("PUT", "latest/"+s.f.cfg.UID, body, nil)
	if err == nil {
		s.pruneIfDue(now)
	}
	return err
}

// rollover flushes finished minute and hour buckets before a sample from a newer bucket is added.
func (s *fbSink) rollover(now time.Time) error {
	minute := now.Truncate(time.Minute)
	if s.min.n > 0 && !minute.Equal(s.minStart) {
		avg := s.min.mean()
		path := fmt.Sprintf("m/%s/%s/%d", s.f.cfg.UID, s.minStart.Format("20060102"), s.minStart.Unix())
		s.hour.add(avg)
		if s.hourStart.IsZero() {
			s.hourStart = s.minStart.Truncate(time.Hour)
		}
		s.min = agg{}
		if _, err := s.f.db("PUT", path, record(avg, map[string]any{"t": s.minStart.Unix()}), nil); err != nil {
			return err
		}
		if hour := minute.Truncate(time.Hour); !hour.Equal(s.hourStart) {
			start, havg := s.hourStart, s.hour.mean()
			s.hour, s.hourStart = agg{}, time.Time{}
			hp := fmt.Sprintf("h/%s/%s/%d", s.f.cfg.UID, start.Format("200601"), start.Unix())
			if _, err := s.f.db("PUT", hp, record(havg, map[string]any{"t": start.Unix()}), nil); err != nil {
				return err
			}
		}
	}
	if s.min.n == 0 || !minute.Equal(s.minStart) {
		s.minStart = minute
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
