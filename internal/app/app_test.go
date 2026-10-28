package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"
)

func fixture(t *testing.T, mode string) (*Store, *Service, *HTTPServer) {
	t.Helper()
	store, err := OpenStore(t.TempDir(), "wg0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	service, err := NewService(store, NewDemoEngine(), mode, "test")
	if err != nil {
		t.Fatal(err)
	}
	var files fs.FS = fstest.MapFS{"index.html": {Data: []byte("hello")}, "app.js": {Data: []byte("app")}}
	h, err := NewHTTP(service, files, "", false)
	if err != nil {
		t.Fatal(err)
	}
	return store, service, h
}
func request(h http.Handler, method, path string, body any, cookie *http.Cookie, csrf string) *httptest.ResponseRecorder {
	var b []byte
	if body != nil {
		b, _ = json.Marshal(body)
	}
	r := httptest.NewRequest(method, "http://127.0.0.1:8787"+path, bytes.NewReader(b))
	r.RemoteAddr = "127.0.0.1:20000"
	if body != nil {
		r.Header.Set("Content-Type", "application/json")
	}
	if cookie != nil {
		r.AddCookie(cookie)
	}
	if csrf != "" {
		r.Header.Set("X-CSRF-Token", csrf)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
func demoLogin(t *testing.T, h http.Handler) (*http.Cookie, string) {
	t.Helper()
	w := request(h, "GET", "/api/session", nil, nil, "")
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var v struct {
		CSRF string `json:"csrf"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
		t.Fatal(err)
	}
	return w.Result().Cookies()[0], v.CSRF
}

func TestDefaultsNeverCreateTelemetry(t *testing.T) {
	store, service, _ := fixture(t, "demo")
	cfg, err := store.Settings()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HistoryEnabled || cfg.AuditEnabled || cfg.MetricsEnabled {
		t.Fatal("optional persistence must be off")
	}
	service.Event("info", "peer.test", "test event")
	for range 5 {
		service.Collect(context.Background())
	}
	if _, err = os.Stat(filepath.Join(store.dir, "telemetry.db")); !os.IsNotExist(err) {
		t.Fatal("default mode must not create telemetry database")
	}
}
func TestSnapshotNeverExposesSecrets(t *testing.T) {
	store, _, server := fixture(t, "demo")
	h := server.Handler()
	cookie, _ := demoLogin(t, h)
	w := request(h, "GET", "/api/snapshot", nil, cookie, "")
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
	peers, _ := store.Peers()
	s, _ := store.Server()
	for _, key := range []string{s.PrivateKey, peers[0].PrivateKey} {
		if strings.Contains(w.Body.String(), key) {
			t.Fatal("private key leaked")
		}
	}
	if strings.Contains(w.Body.String(), "privateKey") || strings.Contains(w.Body.String(), "presharedKey") {
		t.Fatal("private DTO fields leaked")
	}
}
func TestPeerLifecycleAndCSRFFlow(t *testing.T) {
	_, service, server := fixture(t, "demo")
	h := server.Handler()
	cookie, csrf := demoLogin(t, h)
	if w := request(h, "POST", "/api/peers", map[string]any{"name": "Tablet"}, cookie, ""); w.Code != 403 {
		t.Fatalf("CSRF missing: %d", w.Code)
	}
	w := request(h, "POST", "/api/peers", map[string]any{"name": "Tablet"}, cookie, csrf)
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	var p PeerView
	_ = json.Unmarshal(w.Body.Bytes(), &p)
	if p.Address != "10.64.0.8/32" {
		t.Fatal(p.Address)
	}
	config := request(h, "GET", "/api/peers/"+p.ID+"/config", nil, cookie, "")
	if config.Code != 200 || !strings.Contains(config.Body.String(), "PrivateKey = ") || !strings.Contains(config.Body.String(), "Endpoint = vpn.example.com:51820") {
		t.Fatal(config.Code, config.Body.String())
	}
	qr := request(h, "GET", "/api/peers/"+p.ID+"/qr", nil, cookie, "")
	if qr.Code != 200 || !bytes.HasPrefix(qr.Body.Bytes(), []byte{137, 80, 78, 71}) {
		t.Fatal("QR image not generated")
	}
	w = request(h, "PATCH", "/api/peers/"+p.ID, map[string]bool{"enabled": false}, cookie, csrf)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	for _, v := range service.Snapshot().Peers {
		if v.ID == p.ID && v.Enabled {
			t.Fatal("peer not disabled")
		}
	}
	w = request(h, "DELETE", "/api/peers/"+p.ID, nil, cookie, csrf)
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
	for _, v := range service.Snapshot().Peers {
		if v.ID == p.ID {
			t.Fatal("peer not deleted")
		}
	}
}
func TestHostAndOriginChecks(t *testing.T) {
	_, _, server := fixture(t, "demo")
	h := server.Handler()
	for _, tc := range []struct{ host, origin, site string }{{"attacker.example:8787", "", ""}, {"127.0.0.1:8787", "https://attacker.example", ""}, {"127.0.0.1:8787", "", "cross-site"}} {
		r := httptest.NewRequest("GET", "http://127.0.0.1:8787/api/session", nil)
		r.Host = tc.host
		r.Header.Set("Origin", tc.origin)
		r.Header.Set("Sec-Fetch-Site", tc.site)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 403 {
			t.Fatalf("untrusted host/origin accepted: %+v", tc)
		}
	}
}
func TestLiveSetupAndPasswordAuthentication(t *testing.T) {
	_, _, server := fixture(t, "live")
	h := server.Handler()
	if w := request(h, "GET", "/api/snapshot", nil, nil, ""); w.Code != 401 {
		t.Fatal("live API must require auth")
	}
	if w := request(h, "POST", "/api/setup", map[string]string{"token": "wrong", "username": "admin", "password": "test-password-only-123"}, nil, ""); w.Code != 403 {
		t.Fatal("setup token bypass")
	}
	w := request(h, "POST", "/api/setup", map[string]string{"token": server.setupToken, "username": "admin", "password": "test-password-only-123"}, nil, "")
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w = request(h, "POST", "/api/setup", map[string]string{}, nil, ""); w.Code != 409 {
		t.Fatal("setup reusable")
	}
	w = request(h, "POST", "/api/login", map[string]string{"username": "admin", "password": "wrong"}, nil, "")
	if w.Code != 401 {
		t.Fatal("bad password accepted")
	}
	w = request(h, "POST", "/api/login", map[string]string{"username": "admin", "password": "test-password-only-123"}, nil, "")
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	cookie := w.Result().Cookies()[len(w.Result().Cookies())-1]
	var user struct {
		CSRF string `json:"csrf"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &user)
	w = request(h, "POST", "/api/password", map[string]string{"current": "test-password-only-123", "password": "new-test-password-456"}, cookie, user.CSRF)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w = request(h, "GET", "/api/snapshot", nil, cookie, ""); w.Code != 401 {
		t.Fatal("password change must revoke previous sessions")
	}
}
func TestLoginRateLimit(t *testing.T) {
	_, _, server := fixture(t, "live")
	h := server.Handler()
	for range 5 {
		request(h, "POST", "/api/login", map[string]string{"username": "x", "password": "x"}, nil, "")
	}
	if w := request(h, "POST", "/api/login", map[string]string{"username": "x", "password": "x"}, nil, ""); w.Code != 429 {
		t.Fatal("login rate limit absent")
	}
}
func TestTelemetryBudgetRetentionAndClear(t *testing.T) {
	store, _, _ := fixture(t, "demo")
	cfg := DefaultSettings()
	cfg.HistoryEnabled = true
	cfg.AuditEnabled = true
	cfg.BudgetMB = 32
	if err := store.Record(cfg, "audit", Event{ID: "a", Time: time.Now(), Detail: "created"}); err != nil {
		t.Fatal(err)
	}
	store.teleMu.Lock()
	db := store.telemetry
	store.teleMu.Unlock()
	if _, err := db.Exec(`INSERT INTO records(time,kind,value) VALUES(?,?,?)`, time.Now().AddDate(0, 0, -31).Unix(), "audit", `{}`); err != nil {
		t.Fatal(err)
	}
	large := map[string]string{"detail": strings.Repeat("x", 110*1024)}
	for range 100 {
		if err := store.Record(cfg, "audit", large); err != nil {
			t.Fatal(err)
		}
	}
	cfg.BudgetMB = 16
	if err := store.ReconfigureTelemetry(cfg); err != nil {
		t.Fatal("budget reduction failed", err)
	}
	var old int
	if err := db.QueryRow(`SELECT count(*) FROM records WHERE time < ?`, time.Now().AddDate(0, 0, -30).Unix()).Scan(&old); err != nil || old != 0 {
		t.Fatal("retention not applied", err)
	}
	var maxPages int64
	if err := db.QueryRow(`PRAGMA max_page_count`).Scan(&maxPages); err != nil {
		t.Fatal(err)
	}
	if maxPages*4096 > int64(cfg.BudgetMB)*1024*1024/2 {
		t.Fatal("rollback journal not reserved")
	}
	_, size := store.StorageBytes()
	if size > int64(cfg.BudgetMB)*1024*1024 {
		t.Fatalf("budget exceeded: %d", size)
	}
	if err := store.ClearTelemetry(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(store.dir, "telemetry.db")); !os.IsNotExist(err) {
		t.Fatal("telemetry not removed")
	}
	if _, err := store.Server(); err != nil {
		t.Fatal("control data damaged")
	}
}
func TestAuditPersistsAcrossRestart(t *testing.T) {
	store, service, _ := fixture(t, "demo")
	cfg := DefaultSettings()
	cfg.AuditEnabled = true
	if err := store.SaveSettings(cfg); err != nil {
		t.Fatal(err)
	}
	service.Event("info", "peer.create", "persisted test")
	next, err := NewService(store, NewDemoEngine(), "demo", "test")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range next.Snapshot().Events {
		if e.Detail == "persisted test" {
			found = true
		}
	}
	if !found {
		t.Fatal("audit not restored")
	}
}

type failingEngine struct{}

func (failingEngine) Read(context.Context, Server, []Peer) ([]RuntimePeer, bool, error) {
	return nil, true, nil
}
func (failingEngine) Apply(context.Context, Server, []Peer) error {
	return errors.New("simulated apply failure")
}
func TestApplyFailureLeavesControlUntouched(t *testing.T) {
	store, service, _ := fixture(t, "demo")
	before, _ := store.Peers()
	service.engine = failingEngine{}
	if _, err := service.CreatePeer(context.Background(), "Must not persist", nil); err == nil {
		t.Fatal("expected failure")
	}
	after, _ := store.Peers()
	if len(after) != len(before) {
		t.Fatal("failed apply persisted a peer")
	}
}
func TestCounterResetHasNoNegativeOrHugeRate(t *testing.T) {
	if counterRate(1000, 10, 3) != 0 || counterRate(10, 1000, 0) != 0 || counterRate(100, 400, 3) != 100 {
		t.Fatal("counter reset handling broken")
	}
}
func TestImportRejectsHooksAndPreservesPeers(t *testing.T) {
	store, service, _ := fixture(t, "demo")
	s, _ := store.Server()
	peers, _ := store.Peers()
	raw := serverConfig(s, peers, true)
	parsed, imported, err := parseImport(raw, "wg0")
	if err != nil {
		t.Fatal(err)
	}
	if parsed.PublicKey != s.PublicKey || len(imported) != 5 {
		t.Fatal("import mismatch")
	}
	for _, p := range imported {
		if p.PrivateKey != "" {
			t.Fatal("server import cannot invent client private keys")
		}
	}
	for _, bad := range []string{"PostUp = touch /tmp/never", "PreDown = false", "SaveConfig = true"} {
		_, _, err = parseImport(strings.Replace(raw, "[Interface]", "[Interface]\n"+bad, 1), "wg0")
		if err == nil {
			t.Fatal("executable/unsupported directive accepted:", bad)
		}
	}
	if err = service.Import(context.Background(), raw+"\nEndpoint = bad"); err == nil {
		t.Fatal("unsupported peer field accepted")
	}
	after, _ := store.Peers()
	if len(after) != len(peers) {
		t.Fatal("failed import changed control state")
	}
}
func TestDumpParserAndConfigValidation(t *testing.T) {
	s := DefaultServer("wg0")
	priv, pub, _ := newKeyPair()
	raw := fmt.Sprintf("%s\t%s\t51820\toff\n%s\t(none)\t198.51.100.1:42\t10.64.0.2/32\t1700000000\t400\t800\t25\n", priv, s.PublicKey, pub)
	peers, err := parseDump(raw)
	if err != nil || len(peers) != 1 || peers[0].Rx != 400 {
		t.Fatal("dump parse", err)
	}
	if _, err = parseDump("bad\n"); err == nil {
		t.Fatal("invalid dump accepted")
	}
	for _, endpoint := range []string{"ok\nPostUp = bad", "bad;command", "https://vpn.example.com", "bad..example"} {
		v := s
		v.Endpoint = endpoint
		if v.Validate() == nil {
			t.Fatal("invalid endpoint", endpoint)
		}
	}
	if ValidInterface("wg0;touch") || ValidInterface("../wg0") {
		t.Fatal("unsafe interface accepted")
	}
}
func TestModeSeparation(t *testing.T) {
	store, _, _ := fixture(t, "demo")
	if _, err := NewService(store, NewDemoEngine(), "live", "test"); err == nil {
		t.Fatal("live mode must not reuse demo database")
	}
}
func TestConcurrentViewsAndEvents(t *testing.T) {
	_, service, _ := fixture(t, "demo")
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 20 {
				service.Event("info", "test", "test event")
				_ = service.Snapshot()
			}
		}()
	}
	wg.Wait()
	if len(service.Snapshot().Events) > 200 {
		t.Fatal("RAM event buffer unbounded")
	}
}
func TestMetricsDisabledByDefault(t *testing.T) {
	_, _, server := fixture(t, "demo")
	h := server.Handler()
	cookie, _ := demoLogin(t, h)
	if w := request(h, "GET", "/metrics", nil, cookie, ""); w.Code != 404 {
		t.Fatal("metrics must be disabled")
	}
}

func TestMetricsTokenIsReadOnlyAndRotatable(t *testing.T) {
	store, _, server := fixture(t, "demo")
	h := server.Handler()
	cookie, csrf := demoLogin(t, h)
	cfg := DefaultSettings()
	cfg.MetricsEnabled = true
	if err := store.SaveSettings(cfg); err != nil {
		t.Fatal(err)
	}
	w := request(h, "POST", "/api/metrics-token", nil, cookie, csrf)
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	var result struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &result)
	if store.MetricsHash() == result.Token {
		t.Fatal("raw metrics token stored")
	}
	metricRequest := func(path string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", "http://127.0.0.1:8787"+path, nil)
		r.Header.Set("Authorization", "Bearer "+result.Token)
		out := httptest.NewRecorder()
		h.ServeHTTP(out, r)
		return out
	}
	if out := metricRequest("/metrics"); out.Code != 200 || !strings.Contains(out.Body.String(), "wgs_peers") {
		t.Fatal("metrics authentication failed", out.Code)
	}
	if out := metricRequest("/api/snapshot"); out.Code != 401 {
		t.Fatal("metrics token can access management API")
	}
	if w = request(h, "POST", "/api/metrics-token", nil, cookie, csrf); w.Code != 201 {
		t.Fatal(w.Code)
	}
	if out := metricRequest("/metrics"); out.Code != 401 {
		t.Fatal("rotated token still valid")
	}
}
func TestDataDirectoryLock(t *testing.T) {
	dir := t.TempDir()
	release, err := LockData(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if second, err := LockData(dir); err == nil {
		second()
		t.Fatal("second process lock accepted")
	}
}

func TestLiveInterfaceAddressAndMTUValidation(t *testing.T) {
	s := DefaultServer("wg0")
	good := []byte(`[{"mtu":1420,"addr_info":[{"local":"10.64.0.1","prefixlen":24}]}]`)
	if !interfaceMatches(good, s) {
		t.Fatal("matching interface rejected")
	}
	s.MTU = 1380
	if interfaceMatches(good, s) {
		t.Fatal("MTU mismatch accepted")
	}
	s.MTU = 1420
	s.Address = "10.65.0.1/24"
	if interfaceMatches(good, s) {
		t.Fatal("address mismatch accepted")
	}
	if interfaceMatches([]byte(`[]`), s) {
		t.Fatal("missing interface accepted")
	}
}
