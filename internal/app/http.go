package app

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	qrcode "github.com/skip2/go-qrcode"
)

type HTTPServer struct {
	service                  *Service
	store                    *Store
	auth                     *Auth
	mode, setupToken, origin string
	files                    fs.FS
	access                   panelPolicy
}

func NewHTTP(service *Service, files fs.FS, origin string, secure bool) (*HTTPServer, error) {
	s := &HTTPServer{service: service, store: service.store, auth: NewAuth(secure), mode: service.mode, origin: origin, files: files}
	policy, err := readPanelAccess(s.store.dir)
	if err != nil {
		return nil, err
	}
	s.access.config = policy
	if s.mode != "demo" && !s.store.HasAdmin() {
		path := filepath.Join(s.store.dir, "setup-token")
		b, err := os.ReadFile(path)
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		if len(b) > 0 {
			s.setupToken = strings.TrimSpace(string(b))
		} else {
			s.setupToken = token(32)
			if err = atomicFile(path, []byte(s.setupToken+"\n"), 0600); err != nil {
				return nil, err
			}
		}
	}
	return s, nil
}
func jsonResponse(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func apiError(w http.ResponseWriter, status int, msg string) {
	jsonResponse(w, status, map[string]string{"error": msg})
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if strings.Split(r.Header.Get("Content-Type"), ";")[0] != "application/json" {
		apiError(w, 415, "JSON content type required")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 384*1024)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		apiError(w, 400, "Invalid request format or size")
		return false
	}
	if err := d.Decode(&struct{}{}); err != io.EOF {
		apiError(w, 400, "A single JSON object is required")
		return false
	}
	return true
}
func (s *HTTPServer) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/session", s.session)
	mux.HandleFunc("POST /api/setup", s.setup)
	mux.HandleFunc("POST /api/login", s.login)
	mux.HandleFunc("POST /api/logout", s.authorized(s.logout))
	mux.HandleFunc("GET /api/snapshot", s.authorized(s.snapshot))
	mux.HandleFunc("POST /api/peers", s.authorized(s.createPeer))
	mux.HandleFunc("PATCH /api/peers/{id}", s.authorized(s.changePeer))
	mux.HandleFunc("DELETE /api/peers/{id}", s.authorized(s.deletePeer))
	mux.HandleFunc("GET /api/peers/{id}/config", s.authorized(s.peerConfig))
	mux.HandleFunc("GET /api/peers/{id}/qr", s.authorized(s.peerQR))
	mux.HandleFunc("PUT /api/server", s.authorized(s.updateServer))
	mux.HandleFunc("POST /api/import", s.authorized(s.importConfig))
	mux.HandleFunc("PUT /api/settings", s.authorized(s.updateSettings))
	mux.HandleFunc("GET /api/panel-access", s.authorized(s.panelAccess))
	mux.HandleFunc("PUT /api/panel-access", s.authorized(s.updatePanelAccess))
	mux.HandleFunc("DELETE /api/history", s.authorized(s.clearHistory))
	mux.HandleFunc("GET /api/history", s.authorized(s.history))
	mux.HandleFunc("POST /api/password", s.authorized(s.password))
	mux.HandleFunc("POST /api/metrics-token", s.authorized(s.metricsToken))
	mux.HandleFunc("GET /metrics", s.metrics)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { jsonResponse(w, 200, map[string]string{"status": "ok"}) })
	static := http.FileServerFS(s.files)
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" && r.URL.Path != "/app.js" && r.URL.Path != "/style.css" && r.URL.Path != "/design-v2.css" && r.URL.Path != "/favicon.svg" {
			http.NotFound(w, r)
			return
		}
		static.ServeHTTP(w, r)
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' blob: data:; connect-src 'self'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'; object-src 'none'")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		if !s.panelAllowed(r) || !s.allowedRequest(r) {
			apiError(w, 403, "Use the allowed panel address")
			return
		}
		mux.ServeHTTP(w, r)
	})
}
func (s *HTTPServer) allowedRequest(r *http.Request) bool {
	if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
		return false
	}
	if s.origin != "" {
		u, err := url.Parse(s.origin)
		if err != nil || !strings.EqualFold(r.Host, u.Host) {
			return false
		}
		return r.Header.Get("Origin") == "" || r.Header.Get("Origin") == s.origin
	}
	host, _, err := net.SplitHostPort(r.Host)
	if err != nil {
		host = r.Host
	}
	ip := net.ParseIP(host)
	if host != "localhost" && (ip == nil || (s.mode == "demo" && !ip.IsLoopback())) {
		return false
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		return origin == scheme+"://"+r.Host
	}
	return true
}
func (s *HTTPServer) authorized(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := s.authMutation(w, r); ok {
			next(w, r)
		}
	}
}
func (s *HTTPServer) session(w http.ResponseWriter, r *http.Request) {
	user, ok := s.auth.get(r)
	if !ok && s.mode == "demo" {
		user = s.auth.create(w, "Demo administrator")
		ok = true
	}
	jsonResponse(w, 200, map[string]any{"authenticated": ok, "setupRequired": s.mode != "demo" && !s.store.HasAdmin(), "mode": s.mode, "user": user.User, "csrf": user.CSRF})
}
func (s *HTTPServer) setup(w http.ResponseWriter, r *http.Request) {
	if s.mode == "demo" || s.store.HasAdmin() {
		apiError(w, 409, "Setup already completed")
		return
	}
	if !s.auth.allowed(remoteIP(r)) {
		apiError(w, 429, "Too many attempts; wait one minute")
		return
	}
	var input struct {
		Token    string `json:"token"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decode(w, r, &input) {
		return
	}
	if s.setupToken == "" || subtle.ConstantTimeCompare([]byte(input.Token), []byte(s.setupToken)) != 1 {
		apiError(w, 403, "Invalid setup token")
		return
	}
	if !validName(input.Username) || len(input.Username) > 40 {
		apiError(w, 400, "Invalid username")
		return
	}
	select {
	case s.auth.passwordSlot <- struct{}{}:
		defer func() { <-s.auth.passwordSlot }()
	default:
		apiError(w, 429, "Another verification is in progress")
		return
	}
	hash, err := hashPassword(input.Password)
	if err != nil {
		apiError(w, 400, err.Error())
		return
	}
	if err = s.store.CreateAdmin(input.Username, hash); err != nil {
		apiError(w, 409, "Could not create administrator account")
		return
	}
	_ = os.Remove(filepath.Join(s.store.dir, "setup-token"))
	user := s.auth.create(w, input.Username)
	s.service.Event("info", "auth.setup", "Administrator account created")
	jsonResponse(w, 201, map[string]string{"csrf": user.CSRF})
}
func (s *HTTPServer) login(w http.ResponseWriter, r *http.Request) {
	if !s.auth.allowed(remoteIP(r)) {
		apiError(w, 429, "Too many attempts; wait one minute")
		return
	}
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decode(w, r, &input) {
		return
	}
	select {
	case s.auth.passwordSlot <- struct{}{}:
		defer func() { <-s.auth.passwordSlot }()
	default:
		apiError(w, 429, "Another verification is in progress")
		return
	}
	name, hash, err := s.store.Admin()
	if err != nil || !verifyPassword(hash, input.Password) || subtle.ConstantTimeCompare([]byte(name), []byte(input.Username)) != 1 {
		s.service.Event("warning", "auth.failure", "Failed panel login attempt")
		apiError(w, 401, "Incorrect username or password")
		return
	}
	s.auth.logout(w, r)
	user := s.auth.create(w, name)
	s.service.Event("info", "auth.login", "Administrator signed in")
	jsonResponse(w, 200, map[string]string{"csrf": user.CSRF})
}
func (s *HTTPServer) logout(w http.ResponseWriter, r *http.Request) {
	s.auth.logout(w, r)
	jsonResponse(w, 200, map[string]bool{"ok": true})
}
func (s *HTTPServer) snapshot(w http.ResponseWriter, r *http.Request) {
	jsonResponse(w, 200, s.service.Snapshot())
}
func (s *HTTPServer) createPeer(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name      string     `json:"name"`
		ExpiresAt *time.Time `json:"expiresAt"`
	}
	if !decode(w, r, &input) {
		return
	}
	p, err := s.service.CreatePeer(r.Context(), strings.TrimSpace(input.Name), input.ExpiresAt)
	if err != nil {
		apiError(w, 400, err.Error())
		return
	}
	s.service.Event("info", "peer.create", "Client added: "+p.Name)
	s.service.Collect(r.Context())
	jsonResponse(w, 201, p)
}
func (s *HTTPServer) changePeer(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Enabled *bool `json:"enabled"`
	}
	if !decode(w, r, &input) {
		return
	}
	if input.Enabled == nil {
		apiError(w, 400, "Client access state is required")
		return
	}
	if err := s.service.ChangePeer(r.Context(), r.PathValue("id"), input.Enabled, false); err != nil {
		apiError(w, 400, err.Error())
		return
	}
	s.service.Event("info", "peer.update", "Client access status changed")
	s.service.Collect(r.Context())
	jsonResponse(w, 200, map[string]bool{"ok": true})
}
func (s *HTTPServer) deletePeer(w http.ResponseWriter, r *http.Request) {
	if err := s.service.ChangePeer(r.Context(), r.PathValue("id"), nil, true); err != nil {
		apiError(w, 400, err.Error())
		return
	}
	s.service.Event("warning", "peer.delete", "Client deleted")
	s.service.Collect(r.Context())
	jsonResponse(w, 200, map[string]bool{"ok": true})
}
func (s *HTTPServer) findPeer(id string) (Peer, error) {
	peers, err := s.store.Peers()
	if err != nil {
		return Peer{}, err
	}
	for _, p := range peers {
		if p.ID == id {
			return p, nil
		}
	}
	return Peer{}, errors.New("Client not found")
}
func (s *HTTPServer) peerConfig(w http.ResponseWriter, r *http.Request) {
	p, err := s.findPeer(r.PathValue("id"))
	if err != nil {
		apiError(w, 404, err.Error())
		return
	}
	server, err := s.store.Server()
	if err != nil {
		apiError(w, 500, "Could not read server record")
		return
	}
	text, err := clientConfig(server, p)
	if err != nil {
		apiError(w, 400, err.Error())
		return
	}
	s.service.Event("info", "peer.export", "Client configuration downloaded: "+p.Name)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="wireguard-`+p.ID+`.conf"`)
	_, _ = io.WriteString(w, text)
}
func (s *HTTPServer) peerQR(w http.ResponseWriter, r *http.Request) {
	p, err := s.findPeer(r.PathValue("id"))
	if err != nil {
		apiError(w, 404, err.Error())
		return
	}
	server, err := s.store.Server()
	if err != nil {
		apiError(w, 500, "Could not read server record")
		return
	}
	text, err := clientConfig(server, p)
	if err != nil {
		apiError(w, 400, err.Error())
		return
	}
	png, err := qrcode.Encode(text, qrcode.Medium, 360)
	if err != nil {
		apiError(w, 500, "Could not generate QR code")
		return
	}
	s.service.Event("info", "peer.qr", "Client QR code viewed: "+p.Name)
	w.Header().Set("Content-Type", "image/png")
	_, _ = w.Write(png)
}
func (s *HTTPServer) updateServer(w http.ResponseWriter, r *http.Request) {
	var v Server
	if !decode(w, r, &v) {
		return
	}
	if err := s.service.UpdateServer(r.Context(), v); err != nil {
		apiError(w, 400, err.Error())
		return
	}
	s.service.Event("info", "server.update", "Server settings updated")
	s.service.Collect(r.Context())
	jsonResponse(w, 200, map[string]bool{"ok": true})
}
func (s *HTTPServer) importConfig(w http.ResponseWriter, r *http.Request) {
	var v struct {
		Config string `json:"config"`
	}
	if !decode(w, r, &v) {
		return
	}
	if err := s.service.Import(r.Context(), v.Config); err != nil {
		apiError(w, 400, err.Error())
		return
	}
	s.service.Event("warning", "server.import", "WireGuard configuration imported")
	s.service.Collect(r.Context())
	jsonResponse(w, 200, map[string]bool{"ok": true})
}
func (s *HTTPServer) updateSettings(w http.ResponseWriter, r *http.Request) {
	var cfg Settings
	if !decode(w, r, &cfg) {
		return
	}
	if err := cfg.Validate(); err != nil {
		apiError(w, 400, err.Error())
		return
	}
	s.service.changeMu.Lock()
	err := s.store.ReconfigureTelemetry(cfg)
	if err == nil {
		err = s.store.SaveSettings(cfg)
	}
	s.service.changeMu.Unlock()
	if err != nil {
		apiError(w, 500, "Could not apply storage settings")
		return
	}
	s.service.Event("info", "settings.update", "Monitoring and retention preferences updated")
	s.service.Collect(r.Context())
	jsonResponse(w, 200, map[string]bool{"ok": true})
}
func (s *HTTPServer) clearHistory(w http.ResponseWriter, r *http.Request) {
	s.service.changeMu.Lock()
	err := s.store.ClearTelemetry()
	s.service.changeMu.Unlock()
	if err != nil {
		apiError(w, 500, "Could not clear history")
		return
	}
	s.service.mu.Lock()
	s.service.historyErr = ""
	s.service.mu.Unlock()
	s.service.Collect(r.Context())
	jsonResponse(w, 200, map[string]bool{"ok": true})
}
func (s *HTTPServer) history(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.store.Settings()
	if err != nil {
		apiError(w, 500, "Could not read settings")
		return
	}
	if !cfg.HistoryEnabled {
		apiError(w, 409, "Persistent traffic history is off")
		return
	}
	points, err := s.store.History(360)
	if err != nil {
		apiError(w, 500, "Could not read history")
		return
	}
	jsonResponse(w, 200, points)
}
func (s *HTTPServer) password(w http.ResponseWriter, r *http.Request) {
	if s.mode == "demo" {
		apiError(w, 409, "Password cannot be changed in demo mode")
		return
	}
	if !s.auth.allowed(remoteIP(r)) {
		apiError(w, 429, "Wait one minute")
		return
	}
	var v struct {
		Current  string `json:"current"`
		Password string `json:"password"`
	}
	if !decode(w, r, &v) {
		return
	}
	select {
	case s.auth.passwordSlot <- struct{}{}:
		defer func() { <-s.auth.passwordSlot }()
	default:
		apiError(w, 429, "Another verification is in progress")
		return
	}
	_, hash, err := s.store.Admin()
	if err != nil || !verifyPassword(hash, v.Current) {
		apiError(w, 403, "Incorrect current password")
		return
	}
	next, err := hashPassword(v.Password)
	if err != nil {
		apiError(w, 400, err.Error())
		return
	}
	if err = s.store.ChangePassword(next); err != nil {
		apiError(w, 500, "Could not change password")
		return
	}
	s.auth.mu.Lock()
	s.auth.sessions = map[[32]byte]session{}
	s.auth.mu.Unlock()
	s.auth.logout(w, r)
	s.service.Event("info", "auth.password", "Password changed; all sessions signed out")
	jsonResponse(w, 200, map[string]bool{"ok": true})
}
func (s *HTTPServer) metrics(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.store.Settings()
	if err != nil || !cfg.MetricsEnabled {
		http.NotFound(w, r)
		return
	}
	_, ok := s.auth.get(r)
	if !ok {
		raw := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		digest := sha256.Sum256([]byte(raw))
		expected := s.store.MetricsHash()
		if len(raw) < 32 || len(raw) > 128 || expected == "" || subtle.ConstantTimeCompare([]byte(expected), []byte(hex.EncodeToString(digest[:]))) != 1 {
			apiError(w, 401, "Metrics access token required")
			return
		}
	}
	v := s.service.Snapshot()
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	fmt.Fprintf(w, "# TYPE wgs_peers gauge\nwgs_peers %d\n# TYPE wgs_memory_bytes gauge\nwgs_memory_bytes %d\n", len(v.Peers), v.Resources.AppMemory)
	_, _ = io.WriteString(w, "# TYPE wgs_peer_receive_bytes counter\n# TYPE wgs_peer_transmit_bytes counter\n")
	for _, p := range v.Peers {
		fmt.Fprintf(w, "wgs_peer_receive_bytes{id=%q} %d\nwgs_peer_transmit_bytes{id=%q} %d\n", p.ID, p.RxBytes, p.ID, p.TxBytes)
	}
}

func (s *HTTPServer) metricsToken(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.store.Settings()
	if err != nil || !cfg.MetricsEnabled {
		apiError(w, 409, "Enable and save metrics first")
		return
	}
	raw := token(32)
	hash := sha256.Sum256([]byte(raw))
	if err = s.store.SetMetricsHash(hex.EncodeToString(hash[:])); err != nil {
		apiError(w, 500, "Could not save access token")
		return
	}
	s.service.Event("info", "metrics.token", "Read-only metrics token rotated")
	jsonResponse(w, 201, map[string]string{"token": raw})
}

func Serve(ctx context.Context, handler http.Handler, listen, cert, key string) error {
	loopback, err := LoopbackListener(listen)
	if err != nil {
		return err
	}
	if (cert == "") != (key == "") || (!loopback && cert == "") {
		return errors.New("Non-loopback listeners require HTTPS; TLS certificate and key must be supplied together")
	}
	srv := &http.Server{Addr: listen, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 * 1024}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	}()
	if cert != "" {
		err = srv.ListenAndServeTLS(cert, key)
	} else {
		err = srv.ListenAndServe()
	}
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
