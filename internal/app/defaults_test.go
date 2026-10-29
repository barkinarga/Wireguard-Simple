package app

import (
	"strings"
	"testing"
)

func TestEndpointDetectionNeedsUnambiguousPublicIPv4(t *testing.T) {
	for _, tc := range []struct {
		addresses []string
		want      string
	}{
		{[]string{"127.0.0.1/8", "10.0.0.2/24", "2.31.126.150/24"}, "2.31.126.150"},
		{[]string{"2.31.126.150/24", "2.31.126.150/32"}, "2.31.126.150"},
		{[]string{"2.31.126.150/24", "8.8.8.8/32"}, ""},
		{[]string{"10.0.0.2/24", "100.64.0.2/32", "169.254.1.2/16", "192.0.2.2/24", "::1/128", "2001:4860::1/64"}, ""},
		{nil, ""},
	} {
		if got := publicEndpoint(tc.addresses); got != tc.want {
			t.Fatalf("%v: got %q, want %q", tc.addresses, got, tc.want)
		}
	}
}

func TestEndpointInitializationPreservesSavedValues(t *testing.T) {
	store, err := OpenStore(t.TempDir(), "wg0")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.initializeEndpoint(func(string) string { return "2.31.126.150" }); err != nil {
		t.Fatal(err)
	}
	if err := store.initializeEndpoint(func(string) string { t.Fatal("saved endpoint must not be detected again"); return "" }); err != nil {
		t.Fatal(err)
	}
	s, _ := store.Server()
	if s.Endpoint != "2.31.126.150" {
		t.Fatal(s.Endpoint)
	}
	s.Endpoint = "vpn.example.org"
	if err := store.SaveControl(s, nil); err != nil {
		t.Fatal(err)
	}
	if err := store.initializeEndpoint(func(string) string { t.Fatal("custom endpoint replaced"); return "" }); err != nil {
		t.Fatal(err)
	}
}

func TestSystemDNSParsingExcludesStubsAndDeduplicates(t *testing.T) {
	raw := "# nameserver 8.8.8.8\nnameserver 127.0.0.53\nnameserver 10.0.0.53\nnameserver 10.0.0.53 # duplicate\nnameserver 9.9.9.9\nnameserver ::1\nnameserver fe80::1%eth0\nnameserver bad\n"
	if got := resolvDNS(raw); got != "10.0.0.53, 9.9.9.9" {
		t.Fatal(got)
	}
	if got := resolvDNS("nameserver 127.0.0.53\n"); got != "" {
		t.Fatal(got)
	}
	s := DefaultServer("wg0")
	s.DNS = ""
	if err := s.Validate(); err != nil {
		t.Fatal(err)
	}
	s.Endpoint = "vpn.example.org"
	p := Peer{PrivateKey: s.PrivateKey, Address: "10.64.0.2/32"}
	config, err := clientConfig(s, p)
	if err != nil || strings.Contains(config, "DNS =") {
		t.Fatal(config, err)
	}
}
