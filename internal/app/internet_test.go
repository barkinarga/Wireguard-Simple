package app

import (
	"strings"
	"testing"
)

func TestInternetRulesScopeAndUplink(t *testing.T) {
	p, e := internetRules([]byte(`[{"dev":"eth0","gateway":"192.0.2.1"}]`), "wg0", "10.64.0.1/24")
	if e != nil {
		t.Fatal(e)
	}
	for _, s := range []string{`iifname "wg0"`, "ip saddr 10.64.0.0/24", `oifname "eth0"`, "masquerade", internetOwner} {
		if !strings.Contains(p, s) {
			t.Fatalf("missing scope %q", s)
		}
	}
	for _, r := range []string{`[]`, `[{"dev":"eth0"},{"dev":"eth1"}]`, `[{"dev":"wg0"}]`, `[{"dev":"bad\";flush ruleset"}]`, `[{"nexthops":[]}]`} {
		if _, e := internetRules([]byte(r), "wg0", "10.64.0.0/24"); e == nil {
			t.Fatalf("unsafe route accepted: %s", r)
		}
	}
}
func TestInternetRefusesCustomFirewallsAndUnownedTable(t *testing.T) {
	for _, r := range []string{`{"nftables":[{"chain":{"family":"inet","table":"filter","hook":"forward","policy":"drop"}}]}`, `{"nftables":[{"chain":{"family":"ip","table":"filter","hook":"input"}}]}`, `{"nftables":[{"table":{"family":"ip","name":"wgsimple_internet"}}]}`, `not-json`} {
		if _, e := inspectInternetRules([]byte(r)); e == nil {
			t.Fatalf("unsafe firewall accepted: %s", r)
		}
	}
	for _, r := range []string{`{"nftables":[]}`, `{"nftables":[{"table":{"family":"ip","name":"another_nat"}},{"chain":{"family":"ip","table":"another_nat","hook":"postrouting"}}]}`} {
		owned, e := inspectInternetRules([]byte(r))
		if e != nil || owned {
			t.Fatalf("unexpected result: %v %v", owned, e)
		}
	}
	owned, e := inspectInternetRules([]byte(`{"nftables":[{"table":{"family":"ip","name":"wgsimple_internet","comment":"WireGuard Simple managed IPv4 egress"}},{"chain":{"family":"ip","table":"wgsimple_internet","hook":"postrouting"}}]}`))
	if e != nil || !owned {
		t.Fatalf("owned table not recognized: %v", e)
	}
}
