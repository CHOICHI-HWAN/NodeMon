package main

import (
	"database/sql"
	"embed"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

//go:embed all:webdist
var webFS embed.FS

type config struct {
	addr               string
	dbPath             string
	baseURL            string
	googleClientID     string
	googleClientSecret string
	allowedEmails      map[string]bool
	devLogin           bool
	retention          time.Duration
}

type server struct {
	cfg config
	db  *sql.DB
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func loadConfig() config {
	c := config{
		addr:               getenv("ADDR", ":8080"),
		dbPath:             getenv("DB_PATH", "nodemon.db"),
		baseURL:            strings.TrimRight(getenv("BASE_URL", "http://localhost:8080"), "/"),
		googleClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		googleClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		devLogin:           os.Getenv("DEV_LOGIN") == "1",
		retention:          30 * 24 * time.Hour,
		allowedEmails:      map[string]bool{},
	}
	for _, e := range strings.Split(os.Getenv("ALLOWED_EMAILS"), ",") {
		if e = strings.ToLower(strings.TrimSpace(e)); e != "" {
			c.allowedEmails[e] = true
		}
	}
	return c
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// requireUser wraps a handler that needs a logged-in dashboard user.
func (s *server) requireUser(h func(http.ResponseWriter, *http.Request, *user)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, ok := s.currentUser(r)
		if !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		h(w, r, u)
	}
}

func (s *server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /auth/google", s.handleGoogleLogin)
	mux.HandleFunc("GET /auth/google/callback", s.handleGoogleCallback)
	if s.cfg.devLogin {
		mux.HandleFunc("GET /auth/dev", s.handleDevLogin)
	}
	mux.HandleFunc("POST /auth/logout", s.handleLogout)

	mux.HandleFunc("GET /api/me", s.requireUser(s.handleMe))
	mux.HandleFunc("GET /api/overview", s.requireUser(s.handleOverview))
	mux.HandleFunc("GET /api/hosts/{id}/metrics", s.requireUser(s.handleHostMetrics))
	mux.HandleFunc("DELETE /api/hosts/{id}", s.requireUser(s.handleDeleteHost))
	mux.HandleFunc("POST /api/enroll-codes", s.requireUser(s.handleCreateCode))

	mux.HandleFunc("POST /api/agent/enroll", s.handleAgentEnroll)
	mux.HandleFunc("POST /api/agent/ingest", s.handleAgentIngest)

	sub, err := fs.Sub(webFS, "webdist")
	if err != nil {
		log.Fatal(err)
	}
	files := http.FileServerFS(sub)
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		// SPA fallback: unknown paths serve index.html.
		if r.URL.Path != "/" {
			if f, err := sub.Open(strings.TrimPrefix(r.URL.Path, "/")); err == nil {
				f.Close()
				files.ServeHTTP(w, r)
				return
			}
		}
		r.URL.Path = "/"
		files.ServeHTTP(w, r)
	})
	return mux
}

func (s *server) retentionLoop() {
	for {
		cutoff := time.Now().Add(-s.cfg.retention).Unix()
		s.db.Exec(`DELETE FROM metrics WHERE ts < ?`, cutoff)
		now := time.Now().Unix()
		s.db.Exec(`DELETE FROM sessions WHERE expires_at < ?`, now)
		s.db.Exec(`DELETE FROM enroll_codes WHERE expires_at < ?`, now)
		time.Sleep(time.Hour)
	}
}

func main() {
	cfg := loadConfig()
	db, err := openDB(cfg.dbPath)
	if err != nil {
		log.Fatal(err)
	}
	s := &server{cfg: cfg, db: db}
	go s.retentionLoop()
	if cfg.googleClientID == "" && !cfg.devLogin {
		log.Println("warning: GOOGLE_CLIENT_ID is empty and DEV_LOGIN is off; nobody can log in")
	}
	log.Printf("listening on %s (base %s)", cfg.addr, cfg.baseURL)
	log.Fatal((&http.Server{
		Addr: cfg.addr, Handler: s.routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}).ListenAndServe())
}
