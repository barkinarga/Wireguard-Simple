package app

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func ptr[T any](v T) *T { return &v }

func TestAdvancedClientOptionsExportAndPersistence(t *testing.T) {
	store, service, _ := fixture(t, "demo")
	priv, pub, _ := newKeyPair()
	psk, _, _ := newKeyPair()
	options := &CreatePeerOptions{Address: "10.64.0.42/32", PrivateKey: priv, PresharedKey: psk, ClientConfigOptions: ClientConfigOptions{DNS: ptr("9.9.9.9"), AllowedIPs: "192.168.1.0/24", Endpoint: "vpn.example.org", Port: ptr(443), MTU: ptr(1380), PersistentKeepalive: ptr(0), ListenPort: ptr(12345), FwMark: ptr(uint32(123))}}
	p, err := service.CreatePeerWithOptions(context.Background(), "advanced", nil, options)
	if err != nil {
		t.Fatal(err)
	}
	peers, _ := store.Peers()
	var saved Peer
	for _, candidate := range peers {
		if candidate.ID == p.ID {
			saved = candidate
		}
	}
	if saved.PublicKey != pub || saved.PresharedKey != psk {
		t.Fatal("custom keys were not saved")
	}
	server, _ := store.Server()
	config, err := clientConfig(server, saved)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"Address = 10.64.0.42/32", "PrivateKey = " + priv, "PresharedKey = " + psk, "DNS = 9.9.9.9", "AllowedIPs = 192.168.1.0/24", "Endpoint = vpn.example.org:443", "MTU = 1380", "PersistentKeepalive = 0", "ListenPort = 12345", "FwMark = 123"} {
		if !strings.Contains(config, expected) {
			t.Fatalf("missing %q in %s", expected, config)
		}
	}
	if server.Endpoint != "vpn.example.com" || server.MTU != 1420 {
		t.Fatal("server defaults changed")
	}
	service.Collect(context.Background())
	raw, _ := json.Marshal(service.Snapshot())
	if strings.Contains(string(raw), priv) || strings.Contains(string(raw), psk) {
		t.Fatal("client secrets leaked in snapshot")
	}
	options.PrivateKey = server.PrivateKey
	options.Address = "10.64.0.43/32"
	if _, err := service.CreatePeerWithOptions(context.Background(), "reuse-server-key", nil, options); err == nil {
		t.Fatal("server key reuse allowed")
	}
}

func TestAdvancedClientOptionsRejectInvalidInput(t *testing.T) {
	store, service, _ := fixture(t, "demo")
	server, _ := store.Server()
	peers, _ := store.Peers()
	for _, opts := range []CreatePeerOptions{
		{Address: "10.64.0.1/32"}, {Address: "10.64.0.0/32"}, {Address: "10.64.0.255/32"}, {Address: "10.64.0.2/32"}, {Address: "192.168.1.2/32"}, {Address: "10.64.0.40/24"},
		{ClientConfigOptions: ClientConfigOptions{DNS: ptr("9.9.9.9\nPostUp = bad")}},
		{ClientConfigOptions: ClientConfigOptions{Endpoint: "bad\nPostUp=bad"}},
		{ClientConfigOptions: ClientConfigOptions{AllowedIPs: "0.0.0.0/0\nPostUp=bad"}},
		{ClientConfigOptions: ClientConfigOptions{Port: ptr(0)}},
		{ClientConfigOptions: ClientConfigOptions{MTU: ptr(9000)}},
		{ClientConfigOptions: ClientConfigOptions{ListenPort: ptr(-1)}},
		{ClientConfigOptions: ClientConfigOptions{PersistentKeepalive: ptr(65536)}},
		{PrivateKey: "bad"}, {PresharedKey: "bad"},
	} {
		if _, err := opts.validate(server, peers); err == nil {
			t.Fatalf("invalid options accepted: %+v", opts)
		}
	}
	options := &CreatePeerOptions{ClientConfigOptions: ClientConfigOptions{DNS: ptr("")}}
	p, err := service.CreatePeerWithOptions(context.Background(), "device-dns", nil, options)
	if err != nil {
		t.Fatal(err)
	}
	peers, _ = store.Peers()
	for _, saved := range peers {
		if saved.ID != p.ID {
			continue
		}
		config, err := clientConfig(server, saved)
		if err != nil || strings.Contains(config, "DNS =") {
			t.Fatal(config, err)
		}
	}
}

func TestCreateAdvancedClientAPI(t *testing.T) {
	store, _, h := fixture(t, "demo")
	cookie, csrf := demoLogin(t, h.Handler())
	w := request(h.Handler(), "POST", "/api/peers", map[string]any{"name": "api-advanced", "options": map[string]any{"address": "10.64.0.45/32", "dns": "9.9.9.9", "persistentKeepalive": 0}}, cookie, csrf)
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	peers, _ := store.Peers()
	if peers[len(peers)-1].ClientConfig.PersistentKeepalive == nil || *peers[len(peers)-1].ClientConfig.PersistentKeepalive != 0 {
		t.Fatal("API options ignored")
	}
	w = request(h.Handler(), "POST", "/api/peers", map[string]any{"name": "invalid", "options": map[string]any{"mtu": 12}}, cookie, csrf)
	if w.Code != 400 {
		t.Fatal(w.Code)
	}
	w = request(h.Handler(), "GET", "/api/network-defaults", nil, nil, "")
	if w.Code != 401 {
		t.Fatal("network defaults must require authentication", w.Code)
	}
}
