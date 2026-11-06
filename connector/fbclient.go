// SPDX-License-Identifier: GPL-3.0-or-later
package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	defaultAuthURL  = "https://identitytoolkit.googleapis.com"
	defaultTokenURL = "https://securetoken.googleapis.com"

	// Tokens last an hour; renew a little early so an upload never goes out with one about to expire.
	tokenRenewMargin = 5 * time.Minute
	// maxReply bounds what we read back from Firebase. Our own requests never need more than a short listing.
	maxReply = 4 << 20
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

// fbClient talks to Firebase Auth and the Realtime Database REST API as one device.
type fbClient struct {
	cfg     *fbSaved
	http    *http.Client
	idToken string
	expiry  time.Time
	now     func() time.Time
	// compress sends request bodies gzipped. It is opt-in because Realtime Database does not document support
	// for it: the first upload the server rejects switches it off for good and the same data is resent plain.
	compress bool
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

func gzipBytes(b []byte) []byte {
	var buf bytes.Buffer
	w, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	w.Write(b)
	w.Close()
	return buf.Bytes()
}

// explainAuthError turns the terse codes Firebase Auth returns into what the person should actually do.
func explainAuthError(status, body string) string {
	switch {
	case strings.Contains(body, "ADMIN_ONLY_OPERATION"), strings.Contains(body, "OPERATION_NOT_ALLOWED"):
		return "anonymous sign-in is not enabled for this Firebase project. Turn it on in the Firebase console: Authentication > Sign-in method > Anonymous"
	case strings.Contains(body, "API key not valid"), strings.Contains(body, "API_KEY_INVALID"):
		return "the Firebase API key was not accepted. Check --firebase-api-key against the web app settings of the project"
	case strings.Contains(body, "TOO_MANY_ATTEMPTS_TRY_LATER"):
		return "Firebase is limiting sign-in attempts from this network. Wait a few minutes and try again"
	case strings.Contains(body, "INVALID_REFRESH_TOKEN"), strings.Contains(body, "TOKEN_EXPIRED"), strings.Contains(body, "USER_NOT_FOUND"):
		return "this computer's saved credential is no longer valid (the computer may have been removed). Register it again with a new code"
	}
	if len(body) > 300 {
		body = body[:300] + "..."
	}
	return strings.TrimSpace(status + " " + body)
}

func (f *fbClient) postAuth(endpoint string, body io.Reader, contentType string, out any) error {
	resp, err := f.http.Post(endpoint, contentType, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, maxReply))
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("firebase auth: %s", explainAuthError(resp.Status, string(b)))
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
	secs, err := strconv.Atoi(expiresIn)
	if err != nil || secs <= 0 {
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
	if f.idToken == "" || f.now().After(f.expiry.Add(-tokenRenewMargin)) {
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
	if method != http.MethodGet {
		// By default the database echoes every write back; silent skips that download (it counts against the free quota).
		merged := url.Values{"print": {"silent"}}
		for k, v := range extra {
			merged[k] = v
		}
		extra = merged
	}
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
		encoded := false
		if body != nil {
			b, _ := json.Marshal(body)
			if f.compress && method != http.MethodGet {
				b, encoded = gzipBytes(b), true
			}
			rd = bytes.NewReader(b)
		}
		req, _ := http.NewRequest(method, u, rd)
		if encoded {
			req.Header.Set("Content-Encoding", "gzip")
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err := f.http.Do(req)
		if err != nil {
			return nil, err
		}
		b, _ := io.ReadAll(io.LimitReader(resp.Body, maxReply))
		resp.Body.Close()
		switch {
		case resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent:
			return b, nil
		case encoded && (resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusUnsupportedMediaType):
			// The server did not understand the compressed body. Stop compressing and resend the same data plain.
			f.compress = false
			log.Printf("the database does not accept compressed uploads; sending them uncompressed from now on")
			return f.db(method, path, body, extra)
		case resp.StatusCode == http.StatusUnauthorized && attempt == 0:
			f.idToken = "" // force a fresh token, then retry once
		case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
			return nil, errDenied
		default:
			return nil, fmt.Errorf("database %s %s: %s", method, path, resp.Status)
		}
	}
}
