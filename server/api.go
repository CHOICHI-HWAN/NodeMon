package main

import (
	"database/sql"
	"net/http"
	"strconv"
	"time"
)

const onlineWindow = 60 * time.Second

func (s *server) handleMe(w http.ResponseWriter, r *http.Request, u *user) {
	writeJSON(w, 200, u)
}

type hostOverview struct {
	ID       int64      `json:"id"`
	Name     string     `json:"name"`
	OS       string     `json:"os"`
	Online   bool       `json:"online"`
	LastSeen int64      `json:"last_seen"`
	Latest   *sample    `json:"latest"`
	CPUSpark []*float64 `json:"cpu_spark"`
}

func (s *server) handleOverview(w http.ResponseWriter, r *http.Request, u *user) {
	rows, err := s.db.Query(`SELECT id, name, os, last_seen FROM hosts WHERE user_id = ? ORDER BY name`, u.ID)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "db error"})
		return
	}
	hosts := []*hostOverview{}
	for rows.Next() {
		h := &hostOverview{}
		if err := rows.Scan(&h.ID, &h.Name, &h.OS, &h.LastSeen); err != nil {
			rows.Close()
			writeJSON(w, 500, map[string]string{"error": "db error"})
			return
		}
		hosts = append(hosts, h)
	}
	rows.Close()

	now := time.Now()
	for _, h := range hosts {
		h.Online = now.Sub(time.Unix(h.LastSeen, 0)) < onlineWindow
		var m sample
		err := s.db.QueryRow(`SELECT cpu, mem_used, mem_total, disk_used, disk_total, net_rx, net_tx, load1
			FROM metrics WHERE host_id = ? ORDER BY ts DESC LIMIT 1`, h.ID).
			Scan(&m.CPU, &m.MemUsed, &m.MemTotal, &m.DiskUsed, &m.DiskTotal, &m.NetRx, &m.NetTx, &m.Load1)
		if err == nil {
			h.Latest = &m
		}
		h.CPUSpark = s.cpuSpark(h.ID, now)
	}
	writeJSON(w, 200, map[string]any{"hosts": hosts, "now": now.Unix()})
}

// cpuSpark returns 30 one-minute CPU averages ending now; gaps are null.
func (s *server) cpuSpark(hostID int64, now time.Time) []*float64 {
	const n, step = 30, 60
	end := now.Unix() / step * step
	start := end - (n-1)*step
	out := make([]*float64, n)
	rows, err := s.db.Query(`SELECT ts/? AS b, AVG(cpu) FROM metrics WHERE host_id = ? AND ts >= ? GROUP BY b`, step, hostID, start)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var b int64
		var v float64
		if rows.Scan(&b, &v) != nil {
			continue
		}
		if i := int(b - start/step); i >= 0 && i < n {
			out[i] = &v
		}
	}
	return out
}

var ranges = map[string]time.Duration{
	"15m": 15 * time.Minute,
	"1h":  time.Hour,
	"6h":  6 * time.Hour,
	"24h": 24 * time.Hour,
	"7d":  7 * 24 * time.Hour,
	"30d": 30 * 24 * time.Hour,
}

type point struct {
	T         int64   `json:"t"`
	CPU       float64 `json:"cpu"`
	MemUsed   float64 `json:"mem_used"`
	MemTotal  float64 `json:"mem_total"`
	DiskUsed  float64 `json:"disk_used"`
	DiskTotal float64 `json:"disk_total"`
	NetRx     float64 `json:"net_rx"`
	NetTx     float64 `json:"net_tx"`
	Load1     float64 `json:"load1"`
}

// ownsHost reports whether the host exists and belongs to the user.
func (s *server) ownsHost(u *user, idStr string) (int64, bool) {
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return 0, false
	}
	var x int64
	if err := s.db.QueryRow(`SELECT id FROM hosts WHERE id = ? AND user_id = ?`, id, u.ID).Scan(&x); err != nil {
		return 0, false
	}
	return id, true
}

func (s *server) handleHostMetrics(w http.ResponseWriter, r *http.Request, u *user) {
	id, ok := s.ownsHost(u, r.PathValue("id"))
	if !ok {
		writeJSON(w, 404, map[string]string{"error": "not found"})
		return
	}
	rng := r.URL.Query().Get("range")
	d, ok := ranges[rng]
	if !ok {
		d = time.Hour
	}
	bucket := int64(d.Seconds()) / 300 // ~300 points per chart
	if bucket < 1 {
		bucket = 1
	}
	since := time.Now().Add(-d).Unix()
	rows, err := s.db.Query(`SELECT ts/? * ?, AVG(cpu), AVG(mem_used), AVG(mem_total), AVG(disk_used), AVG(disk_total),
		AVG(net_rx), AVG(net_tx), AVG(load1)
		FROM metrics WHERE host_id = ? AND ts >= ? GROUP BY ts/? ORDER BY 1`, bucket, bucket, id, since, bucket)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "db error"})
		return
	}
	defer rows.Close()
	pts := []point{}
	for rows.Next() {
		var p point
		if err := rows.Scan(&p.T, &p.CPU, &p.MemUsed, &p.MemTotal, &p.DiskUsed, &p.DiskTotal, &p.NetRx, &p.NetTx, &p.Load1); err != nil {
			continue
		}
		pts = append(pts, p)
	}
	writeJSON(w, 200, map[string]any{"points": pts, "bucket": bucket})
}

func (s *server) handleDeleteHost(w http.ResponseWriter, r *http.Request, u *user) {
	id, ok := s.ownsHost(u, r.PathValue("id"))
	if !ok {
		writeJSON(w, 404, map[string]string{"error": "not found"})
		return
	}
	if _, err := s.db.Exec(`DELETE FROM hosts WHERE id = ?`, id); err != nil && err != sql.ErrNoRows {
		writeJSON(w, 500, map[string]string{"error": "db error"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
