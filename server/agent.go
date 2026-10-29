package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"net/http"
	"strings"
	"time"
)

const (
	codeTTL     = 15 * time.Minute
	codeAlpha   = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	maxBodySize = 64 << 10
)

func hashKey(k string) string {
	h := sha256.Sum256([]byte(k))
	return hex.EncodeToString(h[:])
}

func newEnrollCode() string {
	b := make([]byte, 12)
	raw := make([]byte, 12)
	if _, err := rand.Read(raw); err != nil {
		panic(err)
	}
	for i := range b {
		b[i] = codeAlpha[int(raw[i])%len(codeAlpha)]
	}
	return string(b[0:4]) + "-" + string(b[4:8]) + "-" + string(b[8:12])
}

func (s *server) handleCreateCode(w http.ResponseWriter, r *http.Request, u *user) {
	code := newEnrollCode()
	exp := time.Now().Add(codeTTL)
	if _, err := s.db.Exec(`INSERT INTO enroll_codes(code, user_id, expires_at) VALUES(?,?,?)`, code, u.ID, exp.Unix()); err != nil {
		writeJSON(w, 500, map[string]string{"error": "db error"})
		return
	}
	writeJSON(w, 200, map[string]any{"code": code, "expires_at": exp.Unix(), "server": s.cfg.baseURL})
}

func (s *server) handleAgentEnroll(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
	var req struct {
		Code string `json:"code"`
		Name string `json:"name"`
		OS   string `json:"os"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "bad request"})
		return
	}
	code := strings.ToUpper(strings.TrimSpace(req.Code))
	name := strings.TrimSpace(req.Name)
	if name == "" || len(name) > 100 || len(req.OS) > 50 {
		writeJSON(w, 400, map[string]string{"error": "invalid name/os"})
		return
	}
	tx, err := s.db.Begin()
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "db error"})
		return
	}
	defer tx.Rollback()
	// Codes are single use: the DELETE ... RETURNING claims it atomically.
	var uid int64
	err = tx.QueryRow(`DELETE FROM enroll_codes WHERE code = ? AND expires_at > ? RETURNING user_id`,
		code, time.Now().Unix()).Scan(&uid)
	if err != nil {
		writeJSON(w, 403, map[string]string{"error": "invalid or expired code"})
		return
	}
	key := "nmk_" + randHex(32)
	var hid int64
	err = tx.QueryRow(`INSERT INTO hosts(user_id, name, os, key_hash, created_at) VALUES(?,?,?,?,?) RETURNING id`,
		uid, name, req.OS, hashKey(key), time.Now().Unix()).Scan(&hid)
	if err != nil || tx.Commit() != nil {
		writeJSON(w, 500, map[string]string{"error": "db error"})
		return
	}
	writeJSON(w, 200, map[string]any{"host_id": hid, "api_key": key})
}

type sample struct {
	CPU       float64 `json:"cpu"`
	MemUsed   float64 `json:"mem_used"`
	MemTotal  float64 `json:"mem_total"`
	DiskUsed  float64 `json:"disk_used"`
	DiskTotal float64 `json:"disk_total"`
	NetRx     float64 `json:"net_rx"`
	NetTx     float64 `json:"net_tx"`
	Load1     float64 `json:"load1"`
}

func (m sample) valid() bool {
	for _, v := range []float64{m.CPU, m.MemUsed, m.MemTotal, m.DiskUsed, m.DiskTotal, m.NetRx, m.NetTx, m.Load1} {
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
			return false
		}
	}
	return m.CPU <= 100*1024
}

func (s *server) handleAgentIngest(w http.ResponseWriter, r *http.Request) {
	auth := r.Header.Get("Authorization")
	key, ok := strings.CutPrefix(auth, "Bearer ")
	if !ok || key == "" {
		writeJSON(w, 401, map[string]string{"error": "missing key"})
		return
	}
	var hid int64
	if err := s.db.QueryRow(`SELECT id FROM hosts WHERE key_hash = ?`, hashKey(key)).Scan(&hid); err != nil {
		writeJSON(w, 401, map[string]string{"error": "invalid key"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
	var m sample
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil || !m.valid() {
		writeJSON(w, 400, map[string]string{"error": "bad sample"})
		return
	}
	now := time.Now().Unix()
	// Timestamps are assigned server-side so a wrong client clock cannot corrupt history.
	if _, err := s.db.Exec(`INSERT OR REPLACE INTO metrics(host_id, ts, cpu, mem_used, mem_total, disk_used, disk_total, net_rx, net_tx, load1)
		VALUES(?,?,?,?,?,?,?,?,?,?)`, hid, now, m.CPU, m.MemUsed, m.MemTotal, m.DiskUsed, m.DiskTotal, m.NetRx, m.NetTx, m.Load1); err != nil {
		writeJSON(w, 500, map[string]string{"error": "db error"})
		return
	}
	s.db.Exec(`UPDATE hosts SET last_seen = ? WHERE id = ?`, now, hid)
	w.WriteHeader(http.StatusNoContent)
}
