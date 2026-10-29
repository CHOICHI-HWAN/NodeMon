package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

// Runs against the Firebase emulators started from the repo root:
//
//	ALLOWED_EMAILS=owner@example.com node firebase/gen-rules.mjs
//	firebase emulators:start --only auth,database --project demo-nodemon
//	NM_FIREBASE_EMULATOR=1 go test ./... -run Emulator
const (
	emuAuth = "http://127.0.0.1:9099/identitytoolkit.googleapis.com"
	emuTok  = "http://127.0.0.1:9099/securetoken.googleapis.com"
	emuDB   = "http://127.0.0.1:9000/?ns=demo-nodemon-default-rtdb"
)

func emuOwnerToken(t *testing.T, email string) (token, uid string) {
	t.Helper()
	idt, _ := json.Marshal(map[string]any{"sub": email, "email": email, "email_verified": true})
	body, _ := json.Marshal(map[string]any{
		"postBody":            "id_token=" + url.QueryEscape(string(idt)) + "&providerId=google.com",
		"requestUri":          "http://localhost",
		"returnIdpCredential": true,
		"returnSecureToken":   true,
	})
	resp, err := http.Post(emuAuth+"/v1/accounts:signInWithIdp?key=x", "application/json", strings.NewReader(string(body)))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var r struct {
		IDToken string `json:"idToken"`
		LocalID string `json:"localId"`
	}
	json.NewDecoder(resp.Body).Decode(&r)
	return r.IDToken, r.LocalID
}

func emuDo(t *testing.T, method, path, token string, body any) []byte {
	t.Helper()
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = strings.NewReader(string(b))
	}
	req, _ := http.NewRequest(method, fmt.Sprintf("http://127.0.0.1:9000/%s.json?ns=demo-nodemon-default-rtdb&auth=%s", path, token), rd)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		t.Fatalf("%s %s: %s %s", method, path, resp.Status, b)
	}
	return b
}

func TestEmulatorEnrollAndRollup(t *testing.T) {
	if os.Getenv("NM_FIREBASE_EMULATOR") != "1" {
		t.Skip("set NM_FIREBASE_EMULATOR=1 with the emulators running")
	}
	ownerTok, ownerUID := emuOwnerToken(t, "owner@example.com")
	code := fmt.Sprintf("T%011d", time.Now().UnixNano()%1e11)
	emuDo(t, "PUT", "pairings/"+code, ownerTok, map[string]any{"owner": ownerUID, "exp": time.Now().UnixMilli() + 600000})

	saved, err := enrollFirebase(newHTTPClient(), fbSaved{APIKey: "x", DatabaseURL: emuDB, AuthURL: emuAuth, TokenURL: emuTok},
		code[:4]+"-"+code[4:8]+"-"+code[8:], "test-pc", "linux") // dashes must be accepted
	if err != nil {
		t.Fatal(err)
	}
	if saved.Owner != ownerUID || saved.UID == "" || saved.RefreshToken == "" {
		t.Fatalf("bad enrolment: %+v", saved)
	}
	if _, err := enrollFirebase(newHTTPClient(), fbSaved{APIKey: "x", DatabaseURL: emuDB, AuthURL: emuAuth, TokenURL: emuTok}, code, "again", "linux"); err == nil {
		t.Fatal("a pairing code must be single use")
	}
	idx := emuDo(t, "GET", "users/"+ownerUID+"/devices/"+saved.UID, ownerTok, nil)
	if !strings.Contains(string(idx), "test-pc") {
		t.Fatalf("device missing from owner's index: %s", idx)
	}

	// Drive the sink with a fake clock so minute/hour boundaries and token refresh are exercised.
	fc := newFBClient(newHTTPClient(), saved)
	base := time.Now().UTC().Truncate(time.Hour).Add(-3 * time.Hour)
	clock := base.Add(10 * time.Minute)
	fc.now = func() time.Time { return clock }
	sink := newFBSink(fc)

	// A bucket far past retention should be pruned on the first push.
	emuStale := map[string]any{"t": 1577836800, "cpu": 1, "mem_used": 1, "mem_total": 1, "disk_used": 1, "disk_total": 1, "net_rx": 0, "net_tx": 0, "load1": 0}
	if _, err := fc.db("PUT", "m/"+saved.UID+"/20200101/1577836800", emuStale, nil); err != nil {
		t.Fatal(err)
	}

	push := func(cpu float64) {
		t.Helper()
		if err := sink.Push(sample{CPU: cpu, MemUsed: 1, MemTotal: 2, DiskUsed: 1, DiskTotal: 2, Load1: 0.5}); err != nil {
			t.Fatal(err)
		}
	}
	for _, cpu := range []float64{10, 20, 30} {
		push(cpu)
		clock = clock.Add(10 * time.Second)
	}
	clock = base.Add(11 * time.Minute)
	push(50) // flushes minute :10 (mean 20)
	clock = base.Add(time.Hour + 5*time.Minute)
	push(60) // flushes minute :11 (mean 50) and the hour (mean 35); token must be refreshed here

	var latest map[string]float64
	json.Unmarshal(emuDo(t, "GET", "latest/"+saved.UID, ownerTok, nil), &latest)
	if latest["cpu"] != 60 || latest["ts"] < 1e12 {
		t.Fatalf("latest wrong (ts should be a server timestamp in ms): %v", latest)
	}

	minKey := fmt.Sprintf("m/%s/%s/%d", saved.UID, base.Add(10*time.Minute).Format("20060102"), base.Add(10*time.Minute).Unix())
	var m map[string]float64
	json.Unmarshal(emuDo(t, "GET", minKey, ownerTok, nil), &m)
	if m["cpu"] != 20 || m["t"] != float64(base.Add(10*time.Minute).Unix()) {
		t.Fatalf("minute bucket wrong: %v", m)
	}
	hourKey := fmt.Sprintf("h/%s/%s/%d", saved.UID, base.Format("200601"), base.Unix())
	var h map[string]float64
	json.Unmarshal(emuDo(t, "GET", hourKey, ownerTok, nil), &h)
	if h["cpu"] != 35 || h["t"] != float64(base.Unix()) {
		t.Fatalf("hour bucket wrong: %v", h)
	}
	if string(emuDo(t, "GET", "m/"+saved.UID+"/20200101", ownerTok, nil)) != "null" {
		t.Fatal("stale bucket was not pruned")
	}

	// Removing the device from the dashboard must cut it off.
	emuDo(t, "DELETE", "devices/"+saved.UID, ownerTok, nil)
	if err := sink.Push(sample{CPU: 1}); err == nil {
		t.Fatal("a removed device should be denied")
	}
}
