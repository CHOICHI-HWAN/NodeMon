package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const (
	sessionCookie = "nm_session"
	stateCookie   = "nm_oauth_state"
	sessionTTL    = 30 * 24 * time.Hour
)

type user struct {
	ID    int64  `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

func randHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

func (s *server) oauthConfig() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     s.cfg.googleClientID,
		ClientSecret: s.cfg.googleClientSecret,
		RedirectURL:  s.cfg.baseURL + "/auth/google/callback",
		Scopes:       []string{"openid", "email", "profile"},
		Endpoint:     google.Endpoint,
	}
}

func (s *server) setCookie(w http.ResponseWriter, name, value string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name: name, Value: value, Path: "/", MaxAge: maxAge,
		HttpOnly: true, SameSite: http.SameSiteLaxMode,
		Secure: strings.HasPrefix(s.cfg.baseURL, "https://"),
	})
}

func (s *server) handleGoogleLogin(w http.ResponseWriter, r *http.Request) {
	if s.cfg.googleClientID == "" {
		http.Error(w, "Google OAuth is not configured", http.StatusServiceUnavailable)
		return
	}
	state := randHex(16)
	s.setCookie(w, stateCookie, state, 600)
	http.Redirect(w, r, s.oauthConfig().AuthCodeURL(state), http.StatusFound)
}

func (s *server) handleGoogleCallback(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie(stateCookie)
	if err != nil || c.Value == "" || c.Value != r.URL.Query().Get("state") {
		http.Error(w, "invalid oauth state", http.StatusBadRequest)
		return
	}
	s.setCookie(w, stateCookie, "", -1)
	cfg := s.oauthConfig()
	tok, err := cfg.Exchange(r.Context(), r.URL.Query().Get("code"))
	if err != nil {
		http.Error(w, "oauth exchange failed", http.StatusBadGateway)
		return
	}
	resp, err := cfg.Client(r.Context(), tok).Get("https://openidconnect.googleapis.com/v1/userinfo")
	if err != nil {
		http.Error(w, "userinfo failed", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	var info struct {
		Sub           string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil || info.Sub == "" {
		http.Error(w, "userinfo invalid", http.StatusBadGateway)
		return
	}
	if !info.EmailVerified || !s.emailAllowed(info.Email) {
		http.Error(w, "this account is not allowed", http.StatusForbidden)
		return
	}
	s.startSession(w, r, info.Sub, info.Email, info.Name)
}

// handleDevLogin is only routed when DEV_LOGIN=1; it lets the app run without Google credentials.
func (s *server) handleDevLogin(w http.ResponseWriter, r *http.Request) {
	email := r.URL.Query().Get("email")
	if email == "" {
		email = "dev@example.com"
	}
	s.startSession(w, r, "dev:"+email, email, email)
}

func (s *server) emailAllowed(email string) bool {
	if len(s.cfg.allowedEmails) == 0 {
		return true
	}
	return s.cfg.allowedEmails[strings.ToLower(email)]
}

func (s *server) startSession(w http.ResponseWriter, r *http.Request, sub, email, name string) {
	now := time.Now().Unix()
	var uid int64
	err := s.db.QueryRow(`
		INSERT INTO users(google_sub, email, name, created_at) VALUES(?,?,?,?)
		ON CONFLICT(google_sub) DO UPDATE SET email=excluded.email, name=excluded.name
		RETURNING id`, sub, email, name, now).Scan(&uid)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	token := randHex(32)
	exp := time.Now().Add(sessionTTL)
	if _, err := s.db.Exec(`INSERT INTO sessions(token, user_id, expires_at) VALUES(?,?,?)`, token, uid, exp.Unix()); err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	s.setCookie(w, sessionCookie, token, int(sessionTTL.Seconds()))
	http.Redirect(w, r, "/", http.StatusFound)
}

func (s *server) currentUser(r *http.Request) (*user, bool) {
	c, err := r.Cookie(sessionCookie)
	if err != nil || c.Value == "" {
		return nil, false
	}
	var u user
	err = s.db.QueryRow(`
		SELECT u.id, u.email, u.name FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE s.token = ? AND s.expires_at > ?`, c.Value, time.Now().Unix()).Scan(&u.ID, &u.Email, &u.Name)
	if err == sql.ErrNoRows || err != nil {
		return nil, false
	}
	return &u, true
}

func (s *server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		s.db.Exec(`DELETE FROM sessions WHERE token = ?`, c.Value)
	}
	s.setCookie(w, sessionCookie, "", -1)
	w.WriteHeader(http.StatusNoContent)
}
