package app

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// PanelAccess controls HTTP admission, independently of VPN client access.
type PanelAccess struct {
	Mode      string   `json:"mode"`
	Allowlist []string `json:"allowlist"`
}

func (p PanelAccess) Validate() error {
	if p.Mode != "public" && p.Mode != "local" && p.Mode != "lan" {
		return errors.New("Panel access must be public, local, or lan")
	}
	if len(p.Allowlist) > 64 {
		return errors.New("At most 64 IP addresses or CIDRs are allowed")
	}
	for _, entry := range p.Allowlist {
		if _, err := netip.ParseAddr(entry); err == nil {
			continue
		}
		if _, err := netip.ParsePrefix(entry); err != nil {
			return errors.New("Allowlist entries must be IP addresses or CIDRs")
		}
	}
	return nil
}

func (p PanelAccess) allows(raw string) bool {
	host, _, err := net.SplitHostPort(raw)
	if err != nil {
		return false
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return false
	}
	ip = ip.Unmap()
	if ip.IsLoopback() {
		return true
	}
	if p.Mode == "local" || (p.Mode == "lan" && !ip.IsPrivate()) {
		return false
	}
	if len(p.Allowlist) == 0 {
		return true
	}
	for _, entry := range p.Allowlist {
		if addr, err := netip.ParseAddr(entry); err == nil && addr.Unmap() == ip {
			return true
		}
		if prefix, err := netip.ParsePrefix(entry); err == nil && prefix.Contains(ip) {
			return true
		}
	}
	return false
}

func readPanelAccess(dir string) (PanelAccess, error) {
	p := PanelAccess{Mode: "public", Allowlist: []string{}}
	data, err := os.ReadFile(filepath.Join(dir, "panel-access.json"))
	if os.IsNotExist(err) {
		return p, nil
	}
	if err != nil {
		return p, err
	}
	if len(data) > 16384 {
		return p, errors.New("Panel access file is too large")
	}
	p = PanelAccess{}
	if err = json.Unmarshal(data, &p); err != nil {
		return p, err
	}
	return p, p.Validate()
}

// ResetPanelAccess is used while the service is stopped and its data lock is held.
func ResetPanelAccess(dir, mode string) error {
	p := PanelAccess{Mode: mode, Allowlist: []string{}}
	if err := p.Validate(); err != nil {
		return err
	}
	return writePanelAccess(dir, p)
}
func writePanelAccess(dir string, p PanelAccess) error {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return atomicFile(filepath.Join(dir, "panel-access.json"), append(data, '\n'), 0600)
}

type panelPolicy struct {
	sync.RWMutex
	config PanelAccess
}

func (s *HTTPServer) panelAllowed(r *http.Request) bool {
	if s.mode == "demo" {
		return true
	}
	s.access.RLock()
	defer s.access.RUnlock()
	// Only the socket peer is trusted. Forwarded headers cannot bypass this policy.
	return s.access.config.allows(r.RemoteAddr)
}
func (s *HTTPServer) panelAccess(w http.ResponseWriter, r *http.Request) {
	s.access.RLock()
	defer s.access.RUnlock()
	jsonResponse(w, 200, s.access.config)
}
func (s *HTTPServer) updatePanelAccess(w http.ResponseWriter, r *http.Request) {
	var p PanelAccess
	if !decode(w, r, &p) {
		return
	}
	for i := range p.Allowlist {
		p.Allowlist[i] = strings.TrimSpace(p.Allowlist[i])
	}
	if err := p.Validate(); err != nil {
		apiError(w, 400, err.Error())
		return
	}
	s.access.Lock()
	defer s.access.Unlock()
	if err := writePanelAccess(s.store.dir, p); err != nil {
		apiError(w, 500, "Could not save panel access")
		return
	}
	s.access.config = p
	jsonResponse(w, 200, map[string]bool{"ok": true})
}
