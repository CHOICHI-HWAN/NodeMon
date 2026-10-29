package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
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
	sink := newFBSink(fc, time.Minute)

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
		push(cpu) // the first push uploads at once; the rest are held back until a minute has passed
		clock = clock.Add(10 * time.Second)
	}
	var first map[string]float64
	json.Unmarshal(emuDo(t, "GET", "latest/"+saved.UID, ownerTok, nil), &first)
	if first["cpu"] != 10 {
		t.Fatalf("only the first sample should have been uploaded so far: %v", first)
	}
	clock = base.Add(11 * time.Minute)
	push(50) // a minute has passed: uploads latest and the finished minute :10 (mean 20) together
	clock = base.Add(time.Hour + 5*time.Minute)
	push(60) // uploads minute :11 (mean 50) and the hour (mean 35); token must be refreshed here

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
	clock = clock.Add(2 * time.Minute) // past the upload interval, so the next push really uploads
	if err := sink.Push(sample{CPU: 1}); err == nil {
		t.Fatal("a removed device should be denied")
	}
}

// A failed upload must not lose finished buckets; they ride along with the next successful one.
func TestSinkKeepsBacklogUntilUploadSucceeds(t *testing.T) {
	var fail bool
	var bodies []map[string]json.RawMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fail {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		if r.Method == http.MethodPatch { // uploads only; ignore the daily prune reads
			var m map[string]json.RawMessage
			json.NewDecoder(r.Body).Decode(&m)
			bodies = append(bodies, m)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	clock := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	fc := newFBClient(srv.Client(), &fbSaved{APIKey: "k", DatabaseURL: srv.URL, UID: "dev1"})
	fc.idToken, fc.expiry, fc.now = "tok", clock.Add(24*time.Hour), func() time.Time { return clock }
	sink := newFBSink(fc, time.Minute)
	sm := sample{CPU: 10, MemTotal: 2, DiskTotal: 2}

	if err := sink.Push(sm); err != nil { // first push uploads immediately
		t.Fatal(err)
	}
	fail = true
	clock = clock.Add(time.Minute)
	if err := sink.Push(sm); err == nil {
		t.Fatal("expected the upload to fail")
	}
	clock = clock.Add(10 * time.Second) // too soon: back-off means no new attempt, so no error and no request
	if err := sink.Push(sm); err != nil {
		t.Fatalf("a push inside the back-off window must not retry: %v", err)
	}
	clock = clock.Add(2 * time.Minute)
	if err := sink.Push(sm); err == nil {
		t.Fatal("expected the upload to fail again")
	}
	fail = false
	clock = clock.Add(5 * time.Minute) // past the doubled wait
	if err := sink.Push(sm); err != nil {
		t.Fatal(err)
	}
	last := bodies[len(bodies)-1]
	buckets := 0
	for k := range last {
		if strings.HasPrefix(k, "m/dev1/") {
			buckets++
		}
	}
	if buckets < 3 {
		t.Fatalf("backlog lost: expected >= 3 minute buckets in one request, got %d (%v)", buckets, last)
	}
	if _, ok := last["latest/dev1"]; !ok || len(bodies) != 2 {
		t.Fatalf("expected latest in a single batched request, bodies=%d", len(bodies))
	}
}

// If multi-path updates are refused, uploads continue path by path; a revoked device stays denied.
func TestSinkFallsBackToSinglePathWrites(t *testing.T) {
	var patches, puts int
	var revoked bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/v1/token") { // a 401 makes the client refresh its token first
			w.Write([]byte(`{"id_token":"tok2","refresh_token":"r2","expires_in":"3600"}`))
			return
		}
		if revoked || r.Method == http.MethodPatch {
			patches++
			http.Error(w, "denied", http.StatusUnauthorized)
			return
		}
		if r.Method == http.MethodPut {
			puts++
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	clock := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	fc := newFBClient(srv.Client(), &fbSaved{APIKey: "k", DatabaseURL: srv.URL, TokenURL: srv.URL, UID: "dev1", RefreshToken: "r"})
	fc.idToken, fc.expiry, fc.now = "tok", clock.Add(24*time.Hour), func() time.Time { return clock }
	sink := newFBSink(fc, time.Minute)
	sm := sample{CPU: 10, MemTotal: 2, DiskTotal: 2}

	if err := sink.Push(sm); err != nil {
		t.Fatalf("fallback should have succeeded: %v", err)
	}
	if !sink.noBatch || puts == 0 {
		t.Fatalf("expected a switch to single-path writes (noBatch=%v puts=%d)", sink.noBatch, puts)
	}
	before := patches
	clock = clock.Add(time.Minute)
	if err := sink.Push(sm); err != nil || patches != before {
		t.Fatalf("after switching it must not try PATCH again (err=%v)", err)
	}

	// A different sink whose device was removed: everything is denied, and that must surface.
	revoked = true
	gone := newFBSink(fc, time.Minute)
	if err := gone.Push(sm); !errors.Is(err, errDenied) {
		t.Fatalf("expected errDenied for a revoked device, got %v", err)
	}
}

// Real averages are long floats; rounding keeps each bucket small, which is what the upload cost depends on.
func TestRecordStaysSmallForRealisticValues(t *testing.T) {
	var a agg
	for _, cpu := range []float64{12.345678901234, 40.1111111111, 7.77777777777} {
		a.add(sample{CPU: cpu, MemUsed: 8123456789.123, MemTotal: 17179869184, DiskUsed: 412345678901.7, DiskTotal: 1e12, NetRx: 123456.789012, NetTx: 7654.321098, Load1: 1.2345678})
	}
	b, _ := json.Marshal(record(a.mean(), map[string]any{"t": int64(1793280000)}))
	if len(b) > 230 {
		t.Fatalf("bucket body is %d bytes: %s", len(b), b)
	}
	if regexp.MustCompile(`\.\d{3,}`).Match(b) { // more than two decimals means a value was not rounded
		t.Fatalf("values were not rounded: %s", b)
	}
}

// A removed device must back off instead of retrying (and refreshing its token) on every measurement.
func TestRejectedDeviceBacksOff(t *testing.T) {
	var requests int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if strings.HasPrefix(r.URL.Path, "/v1/token") {
			w.Write([]byte(`{"id_token":"tok2","refresh_token":"r2","expires_in":"3600"}`))
			return
		}
		http.Error(w, "denied", http.StatusUnauthorized)
	}))
	defer srv.Close()
	clock := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	fc := newFBClient(srv.Client(), &fbSaved{APIKey: "k", DatabaseURL: srv.URL, TokenURL: srv.URL, UID: "d", RefreshToken: "r"})
	fc.idToken, fc.expiry, fc.now = "tok", clock.Add(24*time.Hour), func() time.Time { return clock }
	sink := newFBSink(fc, time.Minute)
	for i := 0; i < 360; i++ { // an hour of 10-second measurements
		sink.Push(sample{CPU: 1})
		clock = clock.Add(10 * time.Second)
	}
	t.Logf("rejected device: %d requests in one hour", requests)
	if requests > 60 {
		t.Fatalf("a rejected device sent %d requests in an hour; back-off is not working", requests)
	}
}
