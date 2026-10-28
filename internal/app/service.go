package app

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"net/netip"
	"runtime"
	"sync"
	"time"
)

type Service struct {
	store                           *Store
	engine                          Engine
	mode, version                   string
	started                         time.Time
	changeMu                        sync.Mutex
	mu                              sync.RWMutex
	snapshot                        Snapshot
	previous                        map[string]RuntimePeer
	lastPoll, lastView, lastPersist time.Time
	lastIdle, lastTotal             uint64
	events                          []Event
	points                          []Point
	historyErr                      string
	wake                            chan struct{}
	lastCleanup                     time.Time
}

func NewService(store *Store, engine Engine, mode, version string) (*Service, error) {
	s := &Service{store: store, engine: engine, mode: mode, version: version, started: time.Now().UTC(), previous: map[string]RuntimePeer{}, events: []Event{}, points: []Point{}, wake: make(chan struct{}, 1)}
	var storedMode string
	modeErr := store.readConfig("mode", &storedMode)
	if modeErr != nil && !errors.Is(modeErr, sql.ErrNoRows) {
		return nil, errors.New("veri klasörünün çalışma modu okunamadı")
	}
	if modeErr == nil && storedMode != mode {
		return nil, errors.New("demo ve canlı mod aynı veri klasörünü kullanamaz")
	}
	if modeErr != nil {
		if _, err := store.db.Exec(`INSERT OR IGNORE INTO config(key,value) VALUES('mode',?)`, `"`+mode+`"`); err != nil {
			return nil, err
		}
	}
	if cfg, err := store.Settings(); err == nil && cfg.AuditEnabled {
		s.events, _ = store.Audits(200)
	}
	peers, err := store.Peers()
	if err != nil {
		return nil, err
	}
	if mode == "demo" && len(peers) == 0 {
		server, err := store.Server()
		if err != nil {
			return nil, err
		}
		server.Name = "İstanbul · edge-01"
		server.Endpoint = "vpn.example.com"
		for i, name := range []string{"MacBook Pro", "iPhone 16", "Ofis bilgisayarı", "Ev sunucusu", "Pixel 9", "Misafir cihazı"} {
			priv, pub, err := newKeyPair()
			if err != nil {
				return nil, err
			}
			peers = append(peers, Peer{ID: token(12), Name: name, Address: fmt.Sprintf("10.64.0.%d/32", i+2), PublicKey: pub, PrivateKey: priv, Enabled: i != 5, CreatedAt: time.Now().Add(-time.Duration(i+1) * 24 * time.Hour).UTC()})
		}
		if err = store.SaveControl(server, peers); err != nil {
			return nil, err
		}
		for i, detail := range []string{"Demo çalışma alanı hazırlandı", "İstemci yapılandırmaları yüklendi", "Canlı izleme başlatıldı"} {
			s.events = append(s.events, Event{ID: token(8), Time: time.Now().Add(-time.Duration(3-i) * time.Minute).UTC(), Level: "info", Action: "system", Detail: detail})
		}
	}
	if mode == "demo" {
		for i := 0; i < 70; i++ {
			s.points = append(s.points, Point{Time: time.Now().Add(-time.Duration(70-i) * 3 * time.Second).UTC(), Rx: 165000 + 35000*math.Sin(float64(i)*.32) + 21000*math.Sin(float64(i)*.83), Tx: 54000 + 18000*math.Sin(float64(i)*.22+1), Active: 4})
		}
	}
	s.Event("info", "system.start", "Yönetim paneli başlatıldı")
	s.Collect(context.Background())
	return s, nil
}

func (s *Service) Run(ctx context.Context) {
	for {
		cfg, err := s.store.Settings()
		if err != nil {
			return
		}
		s.mu.RLock()
		recent := time.Since(s.lastView) < 15*time.Second
		s.mu.RUnlock()
		delay := 30 * time.Second
		if recent {
			delay = 3 * time.Second
		}
		if cfg.HistoryEnabled && time.Duration(cfg.SampleSeconds)*time.Second < delay {
			delay = time.Duration(cfg.SampleSeconds) * time.Second
		}
		t := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			t.Stop()
			return
		case <-s.wake:
			t.Stop()
			s.Collect(ctx)
		case <-t.C:
			s.expire(ctx)
			s.Collect(ctx)
		}
	}
}
func (s *Service) Snapshot() Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	if time.Since(s.lastView) > 15*time.Second {
		select {
		case s.wake <- struct{}{}:
		default:
		}
	}
	s.lastView = time.Now()
	v := s.snapshot
	v.Points = append([]Point{}, s.points...)
	v.Events = append([]Event{}, s.events...)
	return v
}
func (s *Service) Event(level, action, detail string) {
	e := Event{ID: token(10), Time: time.Now().UTC(), Level: level, Action: action, Detail: detail}
	s.mu.Lock()
	s.events = append(s.events, e)
	if len(s.events) > 200 {
		s.events = s.events[len(s.events)-200:]
	}
	s.mu.Unlock()
	cfg, err := s.store.Settings()
	if err == nil {
		if err = s.store.Record(cfg, "audit", e); err != nil {
			s.setHistoryError(err.Error())
		}
	}
}

func (s *Service) setHistoryError(message string) { s.mu.Lock(); s.historyErr = message; s.mu.Unlock() }

func (s *Service) Collect(ctx context.Context) {
	// Shared with mutations so snapshots never combine a new database with an
	// old runtime configuration. One collector serves every browser.
	s.changeMu.Lock()
	defer s.changeMu.Unlock()
	server, err := s.store.Server()
	if err != nil {
		return
	}
	cfg, err := s.store.Settings()
	if err != nil {
		return
	}
	peers, err := s.store.Peers()
	if err != nil {
		return
	}
	rt, up, readErr := s.engine.Read(ctx, server, peers)
	now := time.Now().UTC()
	byKey := map[string]RuntimePeer{}
	for _, p := range rt {
		byKey[p.PublicKey] = p
	}
	warnings := []string{}
	if readErr != nil {
		warnings = append(warnings, readErr.Error())
	}
	if s.mode == "live" && server.Endpoint == "" {
		warnings = append(warnings, "İstemci bağlantıları için sunucu endpoint adresini ayarlayın.")
	}
	views := []PeerView{}
	point := Point{Time: now}
	dt := now.Sub(s.lastPoll).Seconds()
	for _, p := range peers {
		v := PeerView{ID: p.ID, Name: p.Name, Address: p.Address, PublicKey: p.PublicKey, Enabled: p.Enabled, CreatedAt: p.CreatedAt, ExpiresAt: p.ExpiresAt, HasConfig: p.PrivateKey != "", Status: "idle"}
		if !p.Enabled {
			v.Status = "disabled"
		}
		if r, ok := byKey[p.PublicKey]; ok {
			v.Endpoint = r.Endpoint
			v.RxBytes = r.Rx
			v.TxBytes = r.Tx
			if !r.Handshake.IsZero() {
				t := r.Handshake
				v.LastHandshake = &t
			}
			if p.Enabled && now.Sub(r.Handshake) < 3*time.Minute && now.Sub(r.Handshake) >= -time.Minute {
				v.Status = "active"
				point.Active++
			}
			if prev, ok := s.previous[p.PublicKey]; ok && dt > 0 && !s.lastPoll.IsZero() {
				v.RxRate = counterRate(prev.Rx, r.Rx, dt)
				v.TxRate = counterRate(prev.Tx, r.Tx, dt)
			}
			point.Rx += v.RxRate
			point.Tx += v.TxRate
		}
		views = append(views, v)
	}
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	total, used := hostMemory()
	free, diskTotal, _ := diskSpace(s.store.dir)
	controlBytes, teleBytes := s.store.StorageBytes()
	idle, ticks := cpuCounters()
	cpu := 0.0
	if ticks > s.lastTotal && s.lastTotal > 0 {
		cpu = 100 * (1 - float64(idle-s.lastIdle)/float64(ticks-s.lastTotal))
		cpu = math.Max(0, math.Min(100, cpu))
	}
	s.lastIdle, s.lastTotal = idle, ticks
	if cfg.HistoryEnabled && (s.lastPersist.IsZero() || now.Sub(s.lastPersist) >= time.Duration(cfg.SampleSeconds)*time.Second) {
		counters := make([]PeerSample, 0, len(views))
		for _, v := range views {
			counters = append(counters, PeerSample{ID: v.ID, Rx: v.RxBytes, Tx: v.TxBytes, Handshake: v.LastHandshake})
		}
		if err = s.store.Record(cfg, "sample", HistorySample{Point: point, Peers: counters}); err != nil {
			s.setHistoryError(err.Error())
		} else {
			s.setHistoryError("")
		}
		s.lastPersist = now
	}
	if now.Sub(s.lastCleanup) > 10*time.Minute {
		if err = s.store.ReconfigureTelemetry(cfg); err != nil {
			s.setHistoryError("geçmiş temizliği tamamlanamadı")
		}
		s.lastCleanup = now
	}
	s.previous = byKey
	s.lastPoll = now
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.historyErr != "" {
		warnings = append(warnings, s.historyErr)
	}
	s.points = append(s.points, point)
	if len(s.points) > 120 {
		s.points = s.points[len(s.points)-120:]
	}
	s.snapshot = Snapshot{Mode: s.mode, Version: s.version, StartedAt: s.started, UpdatedAt: now, Server: publicServer(server), Settings: cfg, Peers: views, InterfaceUp: up, Warnings: warnings, HistoryPaused: s.historyErr != "", Resources: Resources{AppMemory: mem.Sys, Goroutines: runtime.NumGoroutine(), HostMemoryTotal: total, HostMemoryUsed: used, HostCPU: cpu, DiskFree: free, DiskTotal: diskTotal, ControlBytes: controlBytes, TelemetryBytes: teleBytes}}
}
func counterRate(before, after uint64, seconds float64) float64 {
	if after < before || seconds <= 0 {
		return 0
	}
	return float64(after-before) / seconds
}

func (s *Service) applyControl(ctx context.Context, old Server, oldPeers []Peer, next Server, peers []Peer) error {
	if err := s.engine.Apply(ctx, next, peers); err != nil {
		return err
	}
	if err := s.store.SaveControl(next, peers); err != nil {
		if rollbackErr := s.engine.Apply(ctx, old, oldPeers); rollbackErr != nil {
			return errors.New("kayıt ve geri alma başarısız: yapılandırmayı Linux konsolundan kontrol edin")
		}
		return errors.New("kayıt başarısız; önceki WireGuard yapılandırması geri yüklendi")
	}
	return nil
}

func (s *Service) CreatePeer(ctx context.Context, name string, expires *time.Time) (PeerView, error) {
	s.changeMu.Lock()
	defer s.changeMu.Unlock()
	if !validName(name) {
		return PeerView{}, errors.New("istemci adı 1–64 karakter olmalı")
	}
	if expires != nil && (expires.Before(time.Now()) || expires.After(time.Now().AddDate(5, 0, 0))) {
		return PeerView{}, errors.New("bitiş tarihi gelecekte ve en fazla 5 yıl içinde olmalı")
	}
	server, err := s.store.Server()
	if err != nil {
		return PeerView{}, err
	}
	peers, err := s.store.Peers()
	if err != nil {
		return PeerView{}, err
	}
	if len(peers) >= 1024 {
		return PeerView{}, errors.New("en fazla 1024 istemci destekleniyor")
	}
	for _, p := range peers {
		if p.Name == name {
			return PeerView{}, errors.New("bu isimde bir istemci zaten var")
		}
	}
	address, err := allocateIP(server, peers)
	if err != nil {
		return PeerView{}, err
	}
	priv, pub, err := newKeyPair()
	if err != nil {
		return PeerView{}, err
	}
	psk := make([]byte, 32)
	if _, err = rand.Read(psk); err != nil {
		return PeerView{}, err
	}
	pskText := base64.StdEncoding.EncodeToString(psk)
	p := Peer{ID: token(12), Name: name, Address: address, PrivateKey: priv, PublicKey: pub, PresharedKey: pskText, Enabled: true, CreatedAt: time.Now().UTC(), ExpiresAt: expires}
	next := append(append([]Peer{}, peers...), p)
	if err = s.applyControl(ctx, server, peers, server, next); err != nil {
		return PeerView{}, err
	}
	return PeerView{ID: p.ID, Name: p.Name, Address: p.Address, PublicKey: p.PublicKey, Enabled: true, HasConfig: true, CreatedAt: p.CreatedAt, ExpiresAt: p.ExpiresAt, Status: "idle"}, nil
}
func allocateIP(server Server, peers []Peer) (string, error) {
	prefix, err := netip.ParsePrefix(server.Address)
	if err != nil {
		return "", err
	}
	used := map[netip.Addr]bool{prefix.Addr(): true}
	for _, p := range peers {
		for _, v := range splitCIDRs(p.Address) {
			if pr, e := netip.ParsePrefix(v); e == nil {
				if pr.Bits() != 32 {
					return "", errors.New("yönlendirilmiş alt ağ içeren yapılandırmada otomatik IP dağıtımı kapalı")
				}
				used[pr.Addr()] = true
			}
		}
	}
	for ip := prefix.Masked().Addr().Next(); prefix.Contains(ip) && prefix.Contains(ip.Next()); ip = ip.Next() {
		if !used[ip] {
			return ip.String() + "/32", nil
		}
	}
	return "", errors.New("sunucu alt ağında boş istemci adresi yok")
}
func (s *Service) ChangePeer(ctx context.Context, id string, enabled *bool, remove bool) error {
	s.changeMu.Lock()
	defer s.changeMu.Unlock()
	server, err := s.store.Server()
	if err != nil {
		return err
	}
	old, err := s.store.Peers()
	if err != nil {
		return err
	}
	next := []Peer{}
	found := false
	for _, p := range old {
		if p.ID == id {
			found = true
			if remove {
				continue
			}
			if enabled != nil {
				if *enabled && p.ExpiresAt != nil && p.ExpiresAt.Before(time.Now()) {
					return errors.New("süresi dolan istemci etkinleştirilemez")
				}
				p.Enabled = *enabled
			}
		}
		next = append(next, p)
	}
	if !found {
		return errors.New("istemci bulunamadı")
	}
	return s.applyControl(ctx, server, old, server, next)
}
func (s *Service) UpdateServer(ctx context.Context, input Server) error {
	s.changeMu.Lock()
	defer s.changeMu.Unlock()
	old, err := s.store.Server()
	if err != nil {
		return err
	}
	input.Interface = old.Interface
	input.PrivateKey = old.PrivateKey
	input.PublicKey = old.PublicKey
	if err = input.Validate(); err != nil {
		return err
	}
	peers, err := s.store.Peers()
	if err != nil {
		return err
	}
	if input.Address != old.Address {
		for _, p := range peers {
			for _, v := range splitCIDRs(p.Address) {
				pr, e := netip.ParsePrefix(v)
				network, _ := netip.ParsePrefix(input.Address)
				if e != nil || !network.Contains(pr.Addr()) {
					return errors.New("mevcut istemci adresleri yeni sunucu ağına uymuyor")
				}
			}
		}
	}
	return s.applyControl(ctx, old, peers, input, peers)
}
func (s *Service) Import(ctx context.Context, raw string) error {
	s.changeMu.Lock()
	defer s.changeMu.Unlock()
	old, err := s.store.Server()
	if err != nil {
		return err
	}
	server, peers, err := parseImport(raw, old.Interface)
	if err != nil {
		return err
	}
	server.Name = old.Name
	server.Endpoint = old.Endpoint
	// Import attaches to a running matching interface; it does not start or
	// overwrite a VPN. Every later mutation still checks the public key.
	if s.mode == "live" {
		if _, up, e := s.engine.Read(ctx, server, peers); e != nil || !up {
			return errors.New("içe aktarmadan önce mevcut WireGuard arayüzünü başlatın")
		}
	}
	return s.store.SaveControl(server, peers)
}
func (s *Service) expire(ctx context.Context) {
	peers, err := s.store.Peers()
	if err != nil {
		return
	}
	for _, p := range peers {
		if p.Enabled && p.ExpiresAt != nil && p.ExpiresAt.Before(time.Now()) {
			off := false
			if err = s.ChangePeer(ctx, p.ID, &off, false); err == nil {
				s.Event("info", "peer.expire", "İstemcinin süresi doldu: "+p.Name)
			} else {
				s.Event("error", "peer.expire", "Süresi dolan istemci devre dışı bırakılamadı")
			}
		}
	}
}
