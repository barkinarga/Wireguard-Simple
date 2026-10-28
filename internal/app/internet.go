package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const internetTable = "wgsimple_internet"
const internetOwner = "WireGuard Simple managed IPv4 egress"

// Network bootstrap runs only as a separate root command, never in the panel.
func PrepareInternet(ctx context.Context, store *Store, configDir string) error {
	if runtime.GOOS != "linux" || os.Geteuid() != 0 {
		return errors.New("Network preparation requires root on the target Linux server")
	}
	s, err := store.Server()
	if err != nil {
		return err
	}
	if err = s.Validate(); err != nil {
		return err
	}
	peers, err := store.Peers()
	if err != nil {
		return err
	}
	for _, tool := range []string{"nft", "sysctl", "ip", "wg"} {
		if _, err = exec.LookPath(tool); err != nil {
			return fmt.Errorf("Network preparation requires %s", tool)
		}
	}
	if _, err = command(ctx, "systemctl", []string{"is-active", "--quiet", "firewalld"}, ""); err == nil {
		return errors.New("Active firewalld requires administrator-managed VPN routing; use --no-network and configure forwarding, NAT and UDP access in firewalld")
	}
	if _, e := exec.LookPath("ufw"); e == nil {
		b, e := command(ctx, "ufw", []string{"status"}, "")
		if e != nil {
			return errors.New("Cannot inspect UFW; automatic networking stopped")
		}
		if strings.Contains(string(b), "Status: active") {
			return errors.New("Active UFW requires administrator-managed VPN routing; use --no-network and configure forwarding, NAT and UDP access in UFW")
		}
	}
	if _, e := exec.LookPath("iptables"); e == nil {
		b, e := command(ctx, "iptables", []string{"--version"}, "")
		if e != nil || strings.Contains(string(b), "legacy") {
			return errors.New("Legacy or unreadable iptables requires administrator-managed networking with --no-network")
		}
	}
	rules, err := command(ctx, "nft", []string{"-j", "list", "ruleset"}, "")
	if err != nil {
		return errors.New("Cannot inspect nftables; automatic networking stopped")
	}
	owned, err := inspectInternetRules(rules)
	if err != nil {
		return err
	}
	raw, err := command(ctx, "ip", []string{"-j", "-4", "route", "show", "default"}, "")
	if err != nil {
		return err
	}
	subnet := netip.MustParsePrefix(s.Address).Masked().String()
	plan, err := internetRules(raw, s.Interface, subnet)
	if err != nil {
		return err
	}
	if owned {
		plan = "delete table ip " + internetTable + "\n" + plan
	}
	if _, err = command(ctx, "nft", []string{"--check", "--file", "-"}, plan); err != nil {
		return errors.New("NAT preflight failed; no network changes applied")
	}
	engine := &LiveEngine{ConfigDir: configDir}
	// A stopped configuration may be restored only when it matches our database.
	path := filepath.Join(configDir, s.Interface+".conf")
	if st, e := os.Lstat(path); e == nil {
		if !st.Mode().IsRegular() {
			return errors.New("WireGuard configuration must be a regular file")
		}
		if _, e = command(ctx, "wg", []string{"show", s.Interface, "public-key"}, ""); e != nil {
			b, e := os.ReadFile(path)
			if e != nil {
				return e
			}
			if string(b) != serverConfig(s, peers, true) {
				return errors.New("Stopped WireGuard configuration differs from saved settings; import/reconcile it before automatic startup")
			}
			if e = startManagedInterface(ctx, s, peers, command); e != nil {
				return e
			}
		}
	}
	if err = engine.Apply(ctx, s, peers); err != nil {
		return err
	}
	oldForward, err := os.ReadFile("/proc/sys/net/ipv4/ip_forward")
	if err != nil {
		return err
	}
	sysPath := "/etc/sysctl.d/90-wgsimple-internet.conf"
	if st, e := os.Lstat(sysPath); e == nil && !st.Mode().IsRegular() {
		return errors.New("Forwarding configuration must be a regular file")
	}
	oldFile, readErr := os.ReadFile(sysPath)
	if readErr != nil && !os.IsNotExist(readErr) {
		return readErr
	}
	if readErr == nil && !strings.HasPrefix(string(oldFile), "# WireGuard Simple managed") {
		return errors.New("Existing forwarding file is not owned by WireGuard Simple")
	}
	rollback := func() {
		_, _ = command(context.WithoutCancel(ctx), "sysctl", []string{"-w", "net.ipv4.ip_forward=" + strings.TrimSpace(string(oldForward))}, "")
		if readErr == nil {
			_ = atomicFile(sysPath, oldFile, 0600)
		} else {
			_ = os.Remove(sysPath)
		}
	}
	if err = atomicFile(sysPath, []byte("# WireGuard Simple managed IPv4 internet forwarding\nnet.ipv4.ip_forward = 1\n"), 0600); err != nil {
		return err
	}
	if _, err = command(ctx, "sysctl", []string{"-w", "net.ipv4.ip_forward=1"}, ""); err != nil {
		rollback()
		return err
	}
	if _, err = command(ctx, "nft", []string{"--file", "-"}, plan); err != nil {
		rollback()
		return errors.New("NAT update failed; previous firewall transaction and forwarding setting preserved")
	}
	// nft applies the entire replacement atomically; failure preserves it.
	return nil
}

func inspectInternetRules(raw []byte) (bool, error) {
	var doc struct {
		Nftables []map[string]json.RawMessage `json:"nftables"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return false, errors.New("Cannot parse existing firewall rules")
	}
	owned := false
	for _, obj := range doc.Nftables {
		if b, ok := obj["table"]; ok {
			var t struct{ Family, Name, Comment string }
			if json.Unmarshal(b, &t) != nil {
				return false, errors.New("Invalid firewall table")
			}
			if t.Family == "ip" && t.Name == internetTable {
				if t.Comment != internetOwner {
					return false, errors.New("Reserved NAT table belongs to another configuration")
				}
				owned = true
			}
		}
		if b, ok := obj["chain"]; ok {
			var c struct{ Family, Table, Hook string }
			if json.Unmarshal(b, &c) != nil {
				return false, errors.New("Invalid firewall chain")
			}
			if c.Table == internetTable && c.Family == "ip" {
				continue
			}
			if c.Hook == "input" || c.Hook == "forward" {
				return false, errors.New("Existing input/forward firewall requires administrator-managed VPN rules; use --no-network and configure forwarding, NAT and UDP access explicitly")
			}
		}
	}
	return owned, nil
}
func internetRules(routes []byte, iface, subnet string) (string, error) {
	var entries []struct{ Dev string }
	if err := json.Unmarshal(routes, &entries); err != nil {
		return "", errors.New("Cannot parse default IPv4 route")
	}
	if len(entries) != 1 || !ValidInterface(entries[0].Dev) || entries[0].Dev == iface {
		return "", errors.New("A single unambiguous IPv4 uplink is required for automatic internet access")
	}
	p, err := netip.ParsePrefix(subnet)
	if err != nil || !p.Addr().Is4() || !ValidInterface(iface) {
		return "", errors.New("Invalid VPN network")
	}
	return fmt.Sprintf("table ip %s {\n comment %q\n chain postrouting {\n type nat hook postrouting priority srcnat; policy accept;\n iifname %q ip saddr %s oifname %q counter masquerade\n }\n}\n", internetTable, internetOwner, iface, p.Masked().String(), entries[0].Dev), nil
}
