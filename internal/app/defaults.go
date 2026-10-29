package app

import (
	"context"
	"encoding/json"
	"net"
	"net/netip"
	"os"
	"runtime"
	"strings"
	"time"
)

func (s *Store) initializeEndpoint(detect func(string) string) error {
	server, err := s.Server()
	if err != nil || server.Endpoint != "" {
		return err
	}
	server.Endpoint = detect(server.Interface)
	if server.Endpoint == "" {
		return nil
	}
	raw, err := json.Marshal(server)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`UPDATE config SET value=? WHERE key='server'`, string(raw))
	return err
}

// Use local configuration only: never ask a third-party service for this host's IP.
func detectEndpoint(iface string) string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	var addresses []string
	for _, in := range interfaces {
		if in.Name == iface || in.Flags&net.FlagUp == 0 || in.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := in.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			addresses = append(addresses, addr.String())
		}
	}
	return publicEndpoint(addresses)
}

func publicEndpoint(addresses []string) string {
	seen := map[netip.Addr]bool{}
	for _, raw := range addresses {
		prefix, err := netip.ParsePrefix(raw)
		if err != nil {
			continue
		}
		ip := prefix.Addr()
		if !ip.Is4() || !ip.IsGlobalUnicast() || ip.IsPrivate() {
			continue
		}
		reserved := false
		for _, block := range []string{"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4"} {
			if netip.MustParsePrefix(block).Contains(ip) {
				reserved = true
				break
			}
		}
		if !reserved {
			seen[ip] = true
		}
	}
	// An ambiguous or NAT-only host needs an operator-supplied endpoint.
	if len(seen) == 1 {
		for ip := range seen {
			return ip.String()
		}
	}
	return ""
}

func systemDNS() string {
	if runtime.GOOS != "linux" {
		return ""
	}
	for _, path := range []string{"/etc/resolv.conf", "/run/systemd/resolve/resolv.conf"} {
		if raw, err := os.ReadFile(path); err == nil {
			if dns := resolvDNS(string(raw)); dns != "" {
				return dns
			}
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if raw, err := command(ctx, "resolvectl", []string{"dns"}, ""); err == nil {
		var candidates []string
		for _, line := range strings.Split(string(raw), "\n") {
			if _, after, ok := strings.Cut(line, ":"); ok {
				candidates = append(candidates, strings.Fields(after)...)
			}
		}
		return clientDNS(candidates)
	}
	return ""
}

func resolvDNS(raw string) string {
	var candidates []string
	for _, line := range strings.Split(raw, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "nameserver" {
			candidates = append(candidates, fields[1])
		}
	}
	return clientDNS(candidates)
}

func clientDNS(candidates []string) string {
	var result []string
	seen := map[netip.Addr]bool{}
	for _, raw := range candidates {
		ip, err := netip.ParseAddr(raw)
		// Managed VPN egress is IPv4; never export a local stub or IPv6-only resolver.
		if err != nil || !ip.Is4() || !ip.IsGlobalUnicast() || ip.IsLoopback() || seen[ip] {
			continue
		}
		seen[ip] = true
		result = append(result, ip.String())
		if len(result) == 3 {
			break
		}
	}
	return strings.Join(result, ", ")
}
