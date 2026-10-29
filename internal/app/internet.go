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
func PrepareInternet(ctx context.Context, store *Store, configDir string) (result error) {
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
	f, err := detectFirewall(ctx, command, exec.LookPath)
	if err != nil {
		return err
	}
	defer func() {
		if result != nil {
			result = errors.Join(result, f.rollback(ctx))
		}
	}()
	rules, err := command(ctx, "nft", []string{"-j", "list", "ruleset"}, "")
	if err != nil {
		return errors.New("Cannot inspect nftables; automatic networking stopped")
	}
	owned, err := inspectManagedInternetRules(rules, f.kind)
	if err != nil {
		return err
	}
	raw, err := command(ctx, "ip", []string{"-j", "-4", "route", "show", "default"}, "")
	if err != nil {
		return err
	}
	uplink, err := internetUplink(raw, s.Interface)
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
	if err = f.configure(ctx, s, uplink, 0, true); err != nil {
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
	return inspectManagedInternetRules(raw, "none")
}
func inspectManagedInternetRules(raw []byte, manager string) (bool, error) {
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
			if manager == "ufw" && c.Table == "filter" && (c.Family == "ip" || c.Family == "ip6") {
				var name struct{ Name string }
				if json.Unmarshal(b, &name) != nil {
					return false, errors.New("Invalid UFW chain")
				}
				if name.Name != "INPUT" && name.Name != "FORWARD" && name.Name != "OUTPUT" && !ufwChain(name.Name) {
					return false, errors.New("UFW shares its filter table with custom chains; configure firewall manually")
				}
			}
			if c.Table == internetTable && c.Family == "ip" {
				continue
			}
			if c.Hook == "input" || c.Hook == "forward" {
				if manager == "firewalld" && c.Family == "inet" && c.Table == "firewalld" {
					continue
				}
				if manager == "ufw" && (c.Family == "ip" || c.Family == "ip6") && c.Table == "filter" {
					continue
				}
				return false, errors.New("Existing input/forward firewall requires administrator-managed VPN rules; use --no-network and configure forwarding, NAT and UDP access explicitly")
			}
		}
		if b, ok := obj["rule"]; ok && manager == "ufw" {
			var r struct {
				Family, Table, Chain string
				Expr                 []map[string]json.RawMessage
			}
			if json.Unmarshal(b, &r) != nil {
				return false, errors.New("Invalid UFW rule")
			}
			if r.Table == "filter" && (r.Family == "ip" || r.Family == "ip6") && (r.Chain == "INPUT" || r.Chain == "FORWARD") {
				for _, expression := range r.Expr {
					for kind, value := range expression {
						if kind == "counter" {
							continue
						}
						var jump struct{ Target string }
						if kind != "jump" || json.Unmarshal(value, &jump) != nil || !ufwChain(jump.Target) {
							return false, errors.New("Custom UFW base-chain rules require manual firewall setup")
						}
					}
				}
			}
		}

	}
	return owned, nil
}
func internetUplink(routes []byte, iface string) (string, error) {
	var entries []struct{ Dev string }
	if err := json.Unmarshal(routes, &entries); err != nil {
		return "", errors.New("Cannot parse default IPv4 route")
	}
	if len(entries) != 1 || !ValidInterface(entries[0].Dev) || entries[0].Dev == iface {
		return "", errors.New("A single unambiguous IPv4 uplink is required for automatic internet access")
	}
	return entries[0].Dev, nil
}
func internetRules(routes []byte, iface, subnet string) (string, error) {
	uplink, err := internetUplink(routes, iface)
	if err != nil {
		return "", err
	}
	p, err := netip.ParsePrefix(subnet)
	if err != nil || !p.Addr().Is4() || !ValidInterface(iface) {
		return "", errors.New("Invalid VPN network")
	}
	return fmt.Sprintf("table ip %s {\n comment %q\n chain postrouting {\n type nat hook postrouting priority srcnat; policy accept;\n iifname %q ip saddr %s oifname %q counter masquerade\n }\n}\n", internetTable, internetOwner, iface, p.Masked().String(), uplink), nil
}

func ufwChain(name string) bool {
	return strings.HasPrefix(name, "ufw-") || strings.HasPrefix(name, "ufw6-")
}
