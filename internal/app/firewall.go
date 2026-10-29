package app

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"runtime"
	"slices"
	"strconv"
	"strings"
)

const firewallOwner = "WireGuard Simple managed VPN firewall"
const firewallZone = "wgsimple-vpn"
const firewallPolicy = "wgsimple-egress"

type firewallRunner func(context.Context, string, []string, string) ([]byte, error)
type firewall struct {
	kind     string
	run      firewallRunner
	undo     [][]string
	reloaded bool
}

func detectFirewall(ctx context.Context, run firewallRunner, lookup func(string) (string, error)) (*firewall, error) {
	f := &firewall{kind: "none", run: run}
	if _, err := lookup("ufw"); err == nil {
		raw, err := run(ctx, "ufw", []string{"status"}, "")
		if err != nil {
			return nil, errors.New("Cannot inspect UFW; configure firewall manually")
		}
		switch {
		case strings.HasPrefix(string(raw), "Status: active"):
			f.kind = "ufw"
		case strings.HasPrefix(string(raw), "Status: inactive"):
		default:
			return nil, errors.New("Unknown UFW status; configure firewall manually")
		}
	}
	if _, err := lookup("firewall-cmd"); err == nil {
		if _, err := run(ctx, "firewall-cmd", []string{"--state"}, ""); err == nil {
			if f.kind != "none" {
				return nil, errors.New("UFW and firewalld are both active; configure firewall manually")
			}
			f.kind = "firewalld"
		} else {
			state, e := run(ctx, "systemctl", []string{"show", "--property=ActiveState", "--value", "firewalld.service"}, "")
			if e != nil || strings.TrimSpace(string(state)) != "inactive" {
				return nil, errors.New("Cannot confirm firewalld is inactive; configure firewall manually")
			}
		}
	}
	if _, err := lookup("iptables"); err == nil {
		raw, err := run(ctx, "iptables", []string{"--version"}, "")
		if err != nil || strings.Contains(string(raw), "legacy") {
			return nil, errors.New("Legacy or unreadable iptables requires administrator-managed firewall rules")
		}
	}
	return f, nil
}

func (f *firewall) call(ctx context.Context, args ...string) (string, error) {
	raw, err := f.run(ctx, "firewall-cmd", args, "")
	return strings.TrimSpace(string(raw)), err
}

// Roll back only additions made by this operation, never pre-existing rules.
func (f *firewall) rollback(ctx context.Context) error {
	var result error
	for i := len(f.undo) - 1; i >= 0; i-- {
		cmd := f.undo[i]
		if _, err := f.run(context.WithoutCancel(ctx), cmd[0], cmd[1:], ""); err != nil {
			result = errors.New("Firewall rollback was incomplete; inspect the managed rules")
		}
	}
	if f.reloaded {
		if _, err := f.call(context.WithoutCancel(ctx), "--reload"); err != nil {
			result = errors.New("Firewall rollback reload failed; inspect the managed rules")
		}
	}
	f.undo = nil
	f.reloaded = false
	return result
}

func (f *firewall) ufwAdd(ctx context.Context, args ...string) error {
	raw, err := f.run(ctx, "ufw", args, "")
	if err != nil {
		return err
	}
	// Rules are explicitly IPv4 so a delete cannot remove an existing IPv6 rule.
	if strings.Contains(string(raw), "Rule added") {
		remove := append([]string{"ufw", "--force", "delete"}, args...)
		f.undo = append(f.undo, remove)
	} else if !strings.Contains(string(raw), "Skipping adding existing rule") {
		return errors.New("Unexpected UFW response; inspect firewall rules manually")
	}
	return nil
}

func (f *firewall) fwAdd(ctx context.Context, scope []string, query, add, remove string) error {
	_, err := f.call(ctx, append(append([]string{}, scope...), query)...)
	if err == nil {
		return nil
	}
	var status interface{ ExitCode() int }
	if !errors.As(err, &status) || status.ExitCode() != 1 {
		return errors.New("Cannot query firewalld permission safely; no rule added")
	}
	args := append(append([]string{}, scope...), add)
	if _, err := f.call(ctx, args...); err != nil {
		return err
	}
	f.undo = append(f.undo, append([]string{"firewall-cmd"}, append(append([]string{}, scope...), remove)...))
	return nil
}

func normalizeFirewall(raw string) string {
	lines := strings.Split(raw, "\n")
	for i, line := range lines {
		line = strings.TrimRight(line, " \t\r")
		// Newer versions also annotate headers with (default, active).
		// These runtime markers are not persistent rule configuration.
		if line == strings.TrimSpace(line) && strings.HasSuffix(line, ")") {
			if before, _, ok := strings.Cut(line, " ("); ok {
				line = before
			}
		}
		lines[i] = line
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// New permanent zone/policy objects require reload. Refuse to discard runtime-only state.
func (f *firewall) reloadPreflight(ctx context.Context, uplink string) error {
	for _, args := range [][]string{{"--list-all-zones"}, {"--list-all-policies"}, {"--direct", "--get-all-rules"}, {"--direct", "--get-all-passthroughs"}, {"--get-ipsets"}} {
		live, err := f.call(ctx, args...)
		if err != nil {
			return errors.New("Cannot inspect firewalld runtime configuration")
		}
		saved, err := f.call(ctx, append([]string{"--permanent"}, args...)...)
		if err != nil {
			return errors.New("Cannot inspect permanent firewalld configuration")
		}
		equal := normalizeFirewall(live) == normalizeFirewall(saved)
		if !equal && args[0] == "--list-all-zones" {
			defaultZone, err := f.call(ctx, "--get-default-zone")
			if err == nil {
				equal = equivalentDefaultUplinkBinding(live, saved, defaultZone, uplink)
			}
		}
		if !equal {
			return errors.New("firewalld has runtime-only changes; configure VPN firewall manually without reloading")
		}
		if args[0] == "--get-ipsets" && live != "" {
			return errors.New("Custom firewalld IP sets require manual VPN firewall setup")
		}
	}
	return nil
}

// An unbound uplink already belongs to the default zone. NetworkManager may
// represent that same assignment as a runtime-only explicit interface binding.
// Ignore only this redundant binding, never ports/rules or a different zone.
func equivalentDefaultUplinkBinding(live, saved, defaultZone, uplink string) bool {
	if !ValidInterface(uplink) || !firewallName(defaultZone) {
		return false
	}
	savedZone, err := firewalldInterfaceZone(saved, uplink)
	if err != nil || savedZone != "" {
		return false
	}
	liveZone, err := firewalldInterfaceZone(live, uplink)
	if err != nil || liveZone != defaultZone {
		return false
	}
	lines := strings.Split(normalizeFirewall(live), "\n")
	zone := ""
	for i, line := range lines {
		if line != "" && line == strings.TrimLeft(line, " \t") {
			zone = line
		}
		if zone != defaultZone {
			continue
		}
		before, after, ok := strings.Cut(strings.TrimSpace(line), ":")
		if !ok || before != "interfaces" {
			continue
		}
		var remaining []string
		for _, iface := range strings.Fields(after) {
			if iface != uplink {
				remaining = append(remaining, iface)
			}
		}
		lines[i] = line[:strings.Index(line, ":")+1]
		if len(remaining) > 0 {
			lines[i] += " " + strings.Join(remaining, " ")
		}
	}
	return normalizeFirewall(strings.Join(lines, "\n")) == normalizeFirewall(saved)
}

func firewallName(s string) bool {
	if s == "" || len(s) > 32 {
		return false
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
			return false
		}
	}
	return true
}

func firewallField(raw, key string) string {
	for _, line := range strings.Split(raw, "\n") {
		if before, after, ok := strings.Cut(strings.TrimSpace(line), ":"); ok && before == key {
			return strings.TrimSpace(after)
		}
	}
	return ""
}

func (f *firewall) zoneFor(ctx context.Context, iface string, permanent bool) (string, error) {
	var prefix []string
	if permanent {
		prefix = []string{"--permanent"}
	}
	zones, err := f.call(ctx, append(append([]string{}, prefix...), "--list-all-zones")...)
	if err != nil {
		return "", err
	}
	selected, err := firewalldInterfaceZone(zones, iface)
	if err != nil {
		return "", err
	}
	if selected == "" {
		selected, err = f.call(ctx, "--get-default-zone")
	}
	if err != nil || !firewallName(selected) {
		return "", errors.New("Cannot determine firewalld interface zone")
	}
	return selected, nil
}

func firewalldInterfaceZone(raw, iface string) (string, error) {
	blocks := map[string]string{}
	zone := ""
	for _, line := range strings.Split(normalizeFirewall(raw), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if line == strings.TrimLeft(line, " \t") {
			zone = strings.TrimSpace(line)
			if !firewallName(zone) {
				return "", errors.New("Invalid firewalld zone output")
			}
		}
		if zone == "" {
			return "", errors.New("Incomplete firewalld zone output")
		}
		blocks[zone] += line + "\n"
	}
	if len(blocks) == 0 {
		return "", errors.New("No firewalld zones could be inspected")
	}
	selected := ""
	for name, block := range blocks {
		if firewallField(block, "sources") != "" {
			return "", errors.New("Source-based firewalld zones require manual VPN firewall setup")
		}
		if slices.Contains(strings.Fields(firewallField(block, "interfaces")), iface) {
			if selected != "" {
				return "", errors.New("Ambiguous firewalld interface zone")
			}
			selected = name
		}
	}
	return selected, nil
}

func (f *firewall) forwardingPolicy(ctx context.Context, iface, uplink, uplinkZone, subnet string) error {
	var exists [2]bool
	for i, prefix := range [][]string{nil, {"--permanent"}} {
		zones, err := f.call(ctx, append(append([]string{}, prefix...), "--get-zones")...)
		if err != nil {
			return err
		}
		policies, err := f.call(ctx, append(append([]string{}, prefix...), "--get-policies")...)
		if err != nil {
			return errors.New("This firewalld version does not support VPN policies; configure manually")
		}
		z, p := slices.Contains(strings.Fields(zones), firewallZone), slices.Contains(strings.Fields(policies), firewallPolicy)
		if z != p {
			return errors.New("Reserved firewalld objects are incomplete; reconcile manually")
		}
		exists[i] = z
		if z {
			zone, err := f.call(ctx, append(append([]string{}, prefix...), "--zone="+firewallZone, "--list-all")...)
			if err != nil || firewallField(zone, "target") != "default" || firewallField(zone, "interfaces") != iface || firewallField(zone, "sources") != "" || firewallField(zone, "services") != "" || firewallField(zone, "ports") != "" {
				return errors.New("Managed firewalld zone has custom settings; reconcile manually")
			}
			policy, err := f.call(ctx, append(append([]string{}, prefix...), "--policy="+firewallPolicy, "--list-all")...)
			if err != nil || firewallField(policy, "target") != "CONTINUE" || firewallField(policy, "ingress-zones") != firewallZone || firewallField(policy, "egress-zones") != uplinkZone {
				return errors.New("Managed firewalld policy has custom settings; reconcile manually")
			}
			rules, err := f.call(ctx, append(append([]string{}, prefix...), "--policy="+firewallPolicy, "--list-rich-rules")...)
			if err != nil || rules != fmt.Sprintf(`rule family="ipv4" source address="%s" accept`, subnet) {
				return errors.New("Managed firewalld policy subnet differs; reconcile manually")
			}
		}
		if z && i == 1 {
			for _, object := range []string{"--zone=" + firewallZone, "--policy=" + firewallPolicy} {
				owner, err := f.call(ctx, append(append([]string{}, prefix...), object, "--get-description")...)
				if err != nil || owner != firewallOwner {
					return errors.New("Reserved firewalld objects are not owned by WireGuard Simple")
				}
			}
		}
	}
	if exists[0] != exists[1] {
		return errors.New("Managed firewalld objects differ between runtime and permanent configuration")
	}
	if !exists[0] {
		if err := f.reloadPreflight(ctx, uplink); err != nil {
			return err
		}
		for _, object := range []struct{ kind, name string }{{"zone", firewallZone}, {"policy", firewallPolicy}} {
			if _, err := f.call(ctx, "--permanent", "--new-"+object.kind+"="+object.name); err != nil {
				return err
			}
			f.undo = append(f.undo, []string{"firewall-cmd", "--permanent", "--delete-" + object.kind + "=" + object.name})
			if _, err := f.call(ctx, "--permanent", "--"+object.kind+"="+object.name, "--set-description="+firewallOwner); err != nil {
				return err
			}
		}
	}
	rich := fmt.Sprintf(`rule family="ipv4" source address="%s" accept`, subnet)
	for _, prefix := range [][]string{{"--permanent"}, nil} {
		// New objects become available at runtime only after the checked reload below.
		if prefix == nil && !exists[0] {
			f.reloaded = true
			if _, err := f.call(ctx, "--reload"); err != nil {
				return err
			}
			break
		}
		zone := append(append([]string{}, prefix...), "--zone="+firewallZone)
		if err := f.fwAdd(ctx, zone, "--query-interface="+iface, "--add-interface="+iface, "--remove-interface="+iface); err != nil {
			return err
		}
		policy := append(append([]string{}, prefix...), "--policy="+firewallPolicy)
		for _, v := range []struct{ kind, value string }{{"ingress-zone", firewallZone}, {"egress-zone", uplinkZone}, {"rich-rule", rich}} {
			if err := f.fwAdd(ctx, policy, "--query-"+v.kind+"="+v.value, "--add-"+v.kind+"="+v.value, "--remove-"+v.kind+"="+v.value); err != nil {
				return err
			}
		}
	}
	return nil
}

func (f *firewall) configure(ctx context.Context, s Server, uplink string, panelPort int, forwarding bool) error {
	if !ValidInterface(uplink) || !ValidInterface(s.Interface) || panelPort < 0 || panelPort > 65535 || s.Port < 1 || s.Port > 65535 {
		return errors.New("Invalid firewall parameters")
	}
	prefix, err := netip.ParsePrefix(s.Address)
	if err != nil || !prefix.Addr().Is4() {
		return errors.New("Invalid firewall VPN subnet")
	}
	subnet := prefix.Masked().String()
	switch f.kind {
	case "none":
		return nil
	case "ufw":
		added, err := f.run(ctx, "ufw", []string{"show", "added"}, "")
		if err != nil {
			return err
		}
		for _, line := range strings.Split(string(added), "\n") {
			words := strings.Fields(line)
			for _, word := range words {
				if word == "deny" || word == "reject" || word == "limit" {
					return errors.New("Explicit UFW deny/reject/limit rules require manual review before adding VPN permissions")
				}
			}
		}
		for _, port := range []struct {
			n     int
			proto string
		}{{panelPort, "tcp"}, {s.Port, "udp"}} {
			if port.n == 0 {
				continue
			}
			if err := f.ufwAdd(ctx, "allow", "in", "on", uplink, "proto", port.proto, "from", "0.0.0.0/0", "to", "0.0.0.0/0", "port", strconv.Itoa(port.n)); err != nil {
				return err
			}
		}
		if forwarding {
			return f.ufwAdd(ctx, "route", "allow", "in", "on", s.Interface, "out", "on", uplink, "from", subnet, "to", "0.0.0.0/0")
		}
	case "firewalld":
		zone, err := f.zoneFor(ctx, uplink, false)
		if err != nil {
			return err
		}
		savedZone, err := f.zoneFor(ctx, uplink, true)
		if err != nil || zone != savedZone {
			return errors.New("firewalld uplink zone differs between runtime and permanent settings")
		}
		if forwarding {
			for _, permanent := range []bool{false, true} {
				vpnZone, err := f.zoneFor(ctx, s.Interface, permanent)
				if err != nil {
					return err
				}
				// Never move an explicitly bound VPN out of an administrator's zone.
				var scope []string
				if permanent {
					scope = []string{"--permanent"}
				}
				interfaces, err := f.call(ctx, append(scope, "--zone="+vpnZone, "--list-interfaces")...)
				if err != nil || vpnZone != firewallZone && slices.Contains(strings.Fields(interfaces), s.Interface) {
					return errors.New("VPN interface already has a custom firewalld zone; configure manually")
				}
			}
			if err := f.forwardingPolicy(ctx, s.Interface, uplink, zone, subnet); err != nil {
				return err
			}
		}
		for _, permanent := range []bool{false, true} {
			var scope []string
			if permanent {
				scope = []string{"--permanent"}
			}
			scope = append(scope, "--zone="+zone)
			for _, p := range []struct {
				n     int
				proto string
			}{{panelPort, "tcp"}, {s.Port, "udp"}} {
				if p.n == 0 {
					continue
				}
				value := fmt.Sprintf("%d/%s", p.n, p.proto)
				if err := f.fwAdd(ctx, scope, "--query-port="+value, "--add-port="+value, "--remove-port="+value); err != nil {
					return err
				}
			}
		}
	default:
		return errors.New("Unsupported firewall manager")
	}
	return nil
}

// Port permissions are separate from bootstrap so existing installations can preserve their routing.
func PrepareFirewall(ctx context.Context, store *Store, listen string) (string, error) {
	if runtime.GOOS != "linux" || os.Geteuid() != 0 {
		return "", errors.New("Firewall preparation requires root on the target Linux server")
	}
	s, err := store.Server()
	if err != nil {
		return "", err
	}
	port, err := panelFirewallPort(listen)
	if err != nil {
		return "", err
	}
	f, err := detectFirewall(ctx, command, exec.LookPath)
	if err != nil {
		return "", err
	}
	rules, err := command(ctx, "nft", []string{"-j", "list", "ruleset"}, "")
	if err != nil {
		return "", err
	}
	if _, err := inspectManagedInternetRules(rules, f.kind); err != nil {
		return "", err
	}
	routes, err := command(ctx, "ip", []string{"-j", "-4", "route", "show", "default"}, "")
	if err != nil {
		return "", err
	}
	uplink, err := internetUplink(routes, s.Interface)
	if err != nil {
		return "", err
	}
	if err := f.configure(ctx, s, uplink, port, false); err != nil {
		if undoErr := f.rollback(ctx); undoErr != nil {
			return "", fmt.Errorf("%w; %v", err, undoErr)
		}
		return "", err
	}
	if f.kind == "none" {
		return "No active local filtering firewall; provider firewall may still require port access.", nil
	}
	return fmt.Sprintf("Local %s permissions configured; provider firewall may still require port access.", f.kind), nil
}

func panelFirewallPort(listen string) (int, error) {
	loopback, err := LoopbackListener(listen)
	if err != nil {
		return 0, err
	}
	_, raw, err := net.SplitHostPort(listen)
	if err != nil {
		return 0, err
	}
	port, err := strconv.Atoi(raw)
	if err != nil || port < 1 || port > 65535 {
		return 0, errors.New("Invalid panel firewall port")
	}
	if loopback {
		return 0, nil
	}
	return port, nil
}
