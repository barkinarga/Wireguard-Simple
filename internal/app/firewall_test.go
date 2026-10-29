package app

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
)

type firewallQueryAbsent struct{}

func (firewallQueryAbsent) Error() string { return "permission absent" }
func (firewallQueryAbsent) ExitCode() int { return 1 }

func TestFirewallDetection(t *testing.T) {
	for _, tc := range []struct {
		ufw, fw, want string
		bad           bool
	}{
		{"", "", "none", false}, {"Status: inactive", "", "none", false},
		{"Status: active", "", "ufw", false}, {"", "running", "firewalld", false},
		{"Status: active", "running", "", true}, {"unknown", "", "", true},
	} {
		lookup := func(name string) (string, error) {
			if name == "ufw" && tc.ufw != "" || name == "firewall-cmd" && tc.fw != "" {
				return name, nil
			}
			return "", errors.New("missing")
		}
		run := func(_ context.Context, name string, args []string, _ string) ([]byte, error) {
			if name == "ufw" {
				return []byte(tc.ufw), nil
			}
			return []byte(tc.fw), nil
		}
		f, err := detectFirewall(context.Background(), run, lookup)
		if (err != nil) != tc.bad || err == nil && f.kind != tc.want {
			t.Fatalf("%+v: %v %v", tc, f, err)
		}
	}
}

func TestUFWScopedRulesIdempotenceAndRollback(t *testing.T) {
	ctx := context.Background()
	s := DefaultServer("wg0")
	s.Port = 51999
	var calls []string
	f := &firewall{kind: "ufw", run: func(_ context.Context, name string, args []string, _ string) ([]byte, error) {
		calls = append(calls, name+" "+strings.Join(args, " "))
		if args[0] == "show" {
			return nil, nil
		}
		if args[0] == "--force" {
			return nil, nil
		}
		if slices.Contains(args, "tcp") {
			return []byte("Skipping adding existing rule\n"), nil
		}
		if args[0] == "route" {
			return nil, errors.New("simulated forwarding failure")
		}
		return []byte("Rule added\n"), nil
	}}
	if err := f.configure(ctx, s, "eth0", 9443, true); err == nil {
		t.Fatal("failure not propagated")
	}
	if err := f.rollback(ctx); err != nil {
		t.Fatal(err)
	}
	all := strings.Join(calls, "\n")
	for _, expected := range []string{"proto tcp from 0.0.0.0/0 to 0.0.0.0/0 port 9443", "proto udp from 0.0.0.0/0 to 0.0.0.0/0 port 51999", "route allow in on wg0 out on eth0 from 10.64.0.0/24 to 0.0.0.0/0", "ufw --force delete allow"} {
		if !strings.Contains(all, expected) {
			t.Fatal(expected, all)
		}
	}
	for _, call := range calls {
		if strings.Contains(call, "delete") && strings.Contains(call, "tcp") {
			t.Fatal("pre-existing rule removed", call)
		}
	}
	for _, bad := range []string{"ufw enable", "ufw reset", "ufw default", "port 22"} {
		if strings.Contains(all, bad) {
			t.Fatal(all)
		}
	}
}

func TestUFWCustomDenyFailsBeforeMutation(t *testing.T) {
	f := &firewall{kind: "ufw", run: func(_ context.Context, _ string, args []string, _ string) ([]byte, error) {
		if args[0] != "show" {
			t.Fatal("mutation before preflight", args)
		}
		return []byte("ufw deny 8787/tcp"), nil
	}}
	if err := f.configure(context.Background(), DefaultServer("wg0"), "eth0", 8787, true); err == nil {
		t.Fatal("explicit deny ignored")
	}
}

func TestFirewalldPortScopesAndRollback(t *testing.T) {
	var calls []string
	f := &firewall{kind: "firewalld", run: func(_ context.Context, _ string, args []string, _ string) ([]byte, error) {
		s := strings.Join(args, " ")
		calls = append(calls, s)
		switch {
		case strings.Contains(s, "--list-all-zones"):
			return []byte("public (default, active)\n  interfaces: eth0\n  sources:\ninternal\n  interfaces:\n  sources:"), nil
		case strings.Contains(s, "--zone=public --list-interfaces"):
			return []byte("eth0"), nil
		case strings.Contains(s, "--list-interfaces"), strings.Contains(s, "--list-sources"):
			return nil, nil
		case strings.Contains(s, "--query-port=8787/tcp"):
			return nil, nil // existing in both scopes
		case strings.Contains(s, "--query-port="):
			return nil, firewallQueryAbsent{}
		case s == "--permanent --zone=public --add-port=51820/udp":
			return nil, errors.New("simulated failure")
		default:
			return nil, nil
		}
	}}
	if err := f.configure(context.Background(), DefaultServer("wg0"), "eth0", 8787, false); err == nil {
		t.Fatal("expected failure")
	}
	if err := f.rollback(context.Background()); err != nil {
		t.Fatal(err)
	}
	all := strings.Join(calls, "\n")
	if !strings.Contains(all, "--zone=public --add-port=51820/udp") || !strings.Contains(all, "--zone=public --remove-port=51820/udp") {
		t.Fatal(all)
	}
	if strings.Contains(all, "--remove-port=8787/tcp") || strings.Contains(all, "--reload") {
		t.Fatal("pre-existing rules changed", all)
	}
}

func TestFirewalldRuntimeDifferencesPreventReload(t *testing.T) {
	var calls []string
	f := &firewall{kind: "firewalld", run: func(_ context.Context, _ string, args []string, _ string) ([]byte, error) {
		s := strings.Join(args, " ")
		calls = append(calls, s)
		if s == "--list-all-zones" {
			return []byte("public (active)\n ports: 22/tcp 12345/tcp"), nil
		}
		return []byte("public\n ports: 22/tcp"), nil
	}}
	if err := f.reloadPreflight(context.Background(), "eth0"); err == nil {
		t.Fatal("runtime-only state would be lost")
	}
	if strings.Contains(strings.Join(calls, "\n"), "--reload") {
		t.Fatal(calls)
	}
}

func TestFirewalldDefaultUplinkBindingEquivalence(t *testing.T) {
	saved := "public (default)\n  interfaces:\n  sources:\n  services: ssh\n  ports:\nwork\n  interfaces:\n  sources:\n  services:\n  ports:"
	live := strings.Replace(saved, "  interfaces:", "  interfaces: eth0", 1)
	if !equivalentDefaultUplinkBinding(live, saved, "public", "eth0") {
		t.Fatal("normal NetworkManager default-zone binding rejected")
	}
	if !equivalentDefaultUplinkBinding(live, strings.ReplaceAll(saved, "interfaces:\n", "interfaces: \n"), "public", "eth0") {
		t.Fatal("CentOS trailing spaces in empty fields must not change zone semantics")
	}
	for _, different := range []string{
		strings.Replace(live, "  ports:", "  ports: 12345/tcp", 1),
		strings.Replace(live, "services: ssh", "services: ssh cockpit", 1),
		strings.Replace(live, "interfaces: eth0", "interfaces: eth0 eth1", 1),
	} {
		if equivalentDefaultUplinkBinding(different, saved, "public", "eth0") {
			t.Fatal("real runtime-only change ignored", different)
		}
	}
	if equivalentDefaultUplinkBinding(live, saved, "work", "eth0") {
		t.Fatal("non-default zone binding accepted")
	}
	otherSaved := strings.Replace(saved, "work\n  interfaces:", "work\n  interfaces: eth0", 1)
	if equivalentDefaultUplinkBinding(live, otherSaved, "public", "eth0") {
		t.Fatal("permanent binding to another zone ignored")
	}
}

func TestFirewalldRestoresOnlyObservedRuntimeBinding(t *testing.T) {
	bound := false
	var calls []string
	f := &firewall{restoreInterface: "eth0", restoreZone: "public", reloaded: true, run: func(_ context.Context, _ string, args []string, _ string) ([]byte, error) {
		call := strings.Join(args, " ")
		calls = append(calls, call)
		switch call {
		case "--zone=public --query-interface=eth0":
			if !bound {
				return nil, firewallQueryAbsent{}
			}
		case "--zone=public --add-interface=eth0":
			bound = true
		case "--reload":
			bound = false
		default:
			t.Fatal("Unexpected binding restoration command", call)
		}
		return nil, nil
	}}
	if err := f.restoreRuntimeBinding(context.Background()); err != nil || !bound {
		t.Fatal(err, bound)
	}
	if len(f.undo) != 0 {
		t.Fatal("Restoring a pre-existing binding must not schedule its removal")
	}
	if err := f.rollback(context.Background()); err != nil || !bound {
		t.Fatal("Rollback lost original runtime binding", err, bound)
	}
	for _, call := range calls {
		if strings.Contains(call, "--permanent") || strings.Contains(call, "remove-interface") {
			t.Fatal("Pre-existing binding was persisted or removed", call)
		}
	}
}

func TestFirewalldUnboundInterfaceUsesGlobalDefaultZone(t *testing.T) {
	f := &firewall{run: func(_ context.Context, _ string, args []string, _ string) ([]byte, error) {
		s := strings.Join(args, " ")
		if strings.Contains(s, "--get-default-zone") {
			if strings.Contains(s, "--permanent") {
				t.Fatal("default zone is a standalone global query")
			}
			return []byte("public"), nil
		}
		if strings.Contains(s, "--list-all-zones") {
			return []byte("public (default)\n  interfaces:\n  sources:"), nil
		}
		return nil, nil
	}}
	for _, permanent := range []bool{false, true} {
		zone, err := f.zoneFor(context.Background(), "eth0", permanent)
		if err != nil || zone != "public" {
			t.Fatal(zone, err)
		}
	}
}

func TestManagedFirewallInspectionRejectsMixedCustomFilters(t *testing.T) {
	for _, tc := range []struct {
		manager, raw string
		bad          bool
	}{
		{"ufw", `{"nftables":[{"chain":{"family":"ip","table":"filter","name":"INPUT","hook":"input"}},{"rule":{"family":"ip","table":"filter","chain":"INPUT","expr":[{"counter":{}},{"jump":{"target":"ufw-before-input"}}]}}]}`, false},
		{"ufw", `{"nftables":[{"chain":{"family":"ip6","table":"filter","name":"ufw6-before-input"}}]}`, false},
		{"ufw", `{"nftables":[{"chain":{"family":"ip","table":"filter","name":"DOCKER-USER"}}]}`, true},
		{"ufw", `{"nftables":[{"rule":{"family":"ip","table":"filter","chain":"INPUT","expr":[{"drop":null}]}}]}`, true},
		{"firewalld", `{"nftables":[{"chain":{"family":"inet","table":"firewalld","hook":"forward"}}]}`, false},
		{"firewalld", `{"nftables":[{"chain":{"family":"inet","table":"custom","hook":"input"}}]}`, true},
	} {
		_, err := inspectManagedInternetRules([]byte(tc.raw), tc.manager)
		if (err != nil) != tc.bad {
			t.Fatal(tc, err)
		}
	}
	for _, tc := range []struct {
		listen string
		port   int
	}{{"0.0.0.0:9443", 9443}, {"127.0.0.1:8787", 0}, {"[::1]:8787", 0}} {
		n, err := panelFirewallPort(tc.listen)
		if err != nil || n != tc.port {
			t.Fatal(tc, n, err)
		}
	}
}

func TestFirewalldHeaderMarkersAndAmbiguousQueries(t *testing.T) {
	if normalizeFirewall("public (default, active)\n  ports: 22/tcp") != normalizeFirewall("public (default)\n  ports: 22/tcp") {
		t.Fatal("display markers are not rule changes")
	}
	f := &firewall{run: func(_ context.Context, _ string, args []string, _ string) ([]byte, error) {
		if !strings.Contains(strings.Join(args, " "), "--query-port=") {
			t.Fatal("query failure caused a mutation", args)
		}
		return nil, errors.New("query failed")
	}}
	if err := f.fwAdd(context.Background(), nil, "--query-port=8787/tcp", "--add-port=8787/tcp", "--remove-port=8787/tcp"); err == nil {
		t.Fatal("ambiguous query accepted")
	}
	if len(f.undo) != 0 {
		t.Fatal("query failure scheduled deletion")
	}
}
