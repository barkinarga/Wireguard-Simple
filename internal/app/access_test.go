package app

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestPanelSourcePolicy(t *testing.T) {
	for _, tc := range []struct {
		mode, ip string
		list     []string
		want     bool
	}{
		{"public", "198.51.100.8:9000", nil, true},
		{"local", "198.51.100.8:9000", nil, false},
		{"local", "127.0.0.1:9000", []string{"192.0.2.1"}, true},
		{"local", "[::1]:9000", nil, true},
		{"local", "[::ffff:127.0.0.1]:9000", nil, true},
		{"lan", "10.0.0.9:9000", nil, true},
		{"lan", "172.16.2.1:9000", nil, true},
		{"lan", "[fd00::1]:9000", nil, true},
		{"lan", "198.51.100.8:9000", nil, false},
		{"public", "198.51.100.8:9000", []string{"198.51.100.0/24"}, true},
		{"public", "[::ffff:198.51.100.8]:9000", []string{"198.51.100.8"}, true},
		{"public", "198.51.100.9:9000", []string{"198.51.100.8"}, false},
		{"public", "[2001:db8::5]:9000", []string{"2001:db8::/32"}, true},
		{"lan", "10.0.0.9:9000", []string{"192.168.0.0/16"}, false},
		{"public", "malformed", nil, false},
	} {
		p := PanelAccess{Mode: tc.mode, Allowlist: tc.list}
		if got := p.allows(tc.ip); got != tc.want {
			t.Errorf("%+v: got %v", tc, got)
		}
	}
	for _, p := range []PanelAccess{{Mode: "invalid"}, {Mode: "public", Allowlist: []string{"example.com"}}, {Mode: "public", Allowlist: []string{"10.0.0.1/99"}}} {
		if p.Validate() == nil {
			t.Fatal("invalid policy accepted", p)
		}
	}
}

func TestPanelHTTPAdmissionAndRecovery(t *testing.T) {
	store, _, s := fixture(t, "live")
	h := s.Handler()
	remote := func(path, host, origin, ip string) int {
		r := httptest.NewRequest("GET", "http://198.51.100.1:8787"+path, nil)
		r.Host = host
		r.RemoteAddr = ip
		r.Header.Set("Origin", origin)
		r.Header.Set("X-Forwarded-For", "127.0.0.1")
		r.Header.Set("Forwarded", "for=127.0.0.1")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code
	}
	if remote("/api/session", "198.51.100.1:8787", "http://198.51.100.1:8787", "198.51.100.2:9000") != 200 {
		t.Fatal("public same-origin request rejected")
	}
	if remote("/api/snapshot", "198.51.100.1:8787", "", "198.51.100.2:9000") != 401 {
		t.Fatal("public API bypassed login")
	}
	for _, host := range []string{"evil.example:8787", "localhost.evil:8787"} {
		if remote("/api/session", host, "", "198.51.100.2:9000") != 403 {
			t.Fatal("hostname accepted")
		}
	}
	if remote("/api/session", "198.51.100.1:8787", "http://evil.example", "198.51.100.2:9000") != 403 {
		t.Fatal("cross-origin accepted")
	}
	if w := request(h, "PUT", "/api/panel-access", PanelAccess{Mode: "local"}, nil, ""); w.Code != 401 {
		t.Fatal("unauthenticated policy update")
	}
	// Use a real session cookie to exercise both authentication and CSRF.
	recorder := httptest.NewRecorder()
	session := s.auth.create(recorder, "admin")
	c := recorder.Result().Cookies()[0]
	if w := request(h, "PUT", "/api/panel-access", PanelAccess{Mode: "local"}, c, ""); w.Code != 403 {
		t.Fatal("CSRF bypass")
	}
	if w := request(h, "PUT", "/api/panel-access", PanelAccess{Mode: "local"}, c, session.CSRF); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	for _, path := range []string{"/", "/api/session", "/healthz", "/metrics", "/app.js"} {
		if remote(path, "198.51.100.1:8787", "", "198.51.100.2:9000") != 403 {
			t.Fatal("policy bypass", path)
		}
	}
	if remote("/api/session", "127.0.0.1:8787", "", "127.0.0.1:9000") != 200 {
		t.Fatal("local recovery unavailable")
	}
	p, err := readPanelAccess(store.dir)
	if err != nil || p.Mode != "local" {
		t.Fatal("policy not persisted", err)
	}
	if err := ResetPanelAccess(store.dir, "public"); err != nil {
		t.Fatal(err)
	}
	p, err = readPanelAccess(store.dir)
	if err != nil || p.Mode != "public" || len(p.Allowlist) != 0 {
		t.Fatal("recovery failed", err)
	}
	data, _ := os.ReadFile(filepath.Join(store.dir, "panel-access.json"))
	var out PanelAccess
	if json.Unmarshal(data, &out) != nil {
		t.Fatal("invalid recovery file")
	}
	if err := os.WriteFile(filepath.Join(store.dir, "panel-access.json"), []byte(`{"mode":"broken"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = readPanelAccess(store.dir); err == nil {
		t.Fatal("broken policy should fail closed")
	}
}
