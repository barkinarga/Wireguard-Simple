package app

import (
	"context"
	"errors"
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

func TestLegacyNFTTableCommentRecovery(t *testing.T) {
	missing := `{"nftables":[{"metainfo":{"version":"1.0.2"}},{"table":{"family":"ip","name":"wgsimple_internet","handle":9}},{"chain":{"family":"ip","table":"wgsimple_internet","hook":"postrouting"}}]}`
	owned := `table ip wgsimple_internet {
 comment "WireGuard Simple managed IPv4 egress"
 chain postrouting { }
}`
	for _, tc := range []struct {
		name, raw, text                  string
		queryFails, wantError, wantOwned bool
		calls                            int
	}{
		{name: "Ubuntu 22 JSON omission", raw: missing, text: owned, wantOwned: true, calls: 2},
		{name: "modern JSON", raw: `{"nftables":[{"table":{"family":"ip","name":"wgsimple_internet","comment":"WireGuard Simple managed IPv4 egress"}}]}`, wantOwned: true, calls: 1},
		{name: "fresh ruleset", raw: `{"nftables":[]}`, calls: 1},
		{name: "unrelated table", raw: `{"nftables":[{"table":{"family":"ip","name":"another_nat"}}]}`, calls: 1},
		{name: "missing marker", raw: missing, text: "table ip wgsimple_internet {\n chain postrouting { }\n}", wantError: true, calls: 2},
		{name: "foreign marker", raw: missing, text: strings.Replace(owned, internetOwner, "administrator", 1), wantError: true, calls: 2},
		{name: "nested marker", raw: missing, text: "table ip wgsimple_internet {\n chain postrouting {\n comment \"" + internetOwner + "\"\n}\n}", wantError: true, calls: 2},
		{name: "wrong table", raw: missing, text: strings.Replace(owned, internetTable, "another_nat", 1), wantError: true, calls: 2},
		{name: "read failure", raw: missing, queryFails: true, wantError: true, calls: 2},
		{name: "contradictory JSON", raw: `{"nftables":[{"table":{"family":"ip","name":"wgsimple_internet","comment":"administrator"}}]}`, text: owned, wantError: true, calls: 1},
		{name: "custom filter preserved", raw: strings.Replace(missing, `}]}`, `},{"chain":{"family":"inet","table":"custom","hook":"input"}}]}`, 1), text: owned, wantError: true, calls: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			run := func(_ context.Context, name string, args []string, _ string) ([]byte, error) {
				calls++
				joined := strings.Join(args, " ")
				if name != "nft" {
					t.Fatal(name, args)
				}
				if joined == "-j list ruleset" {
					return []byte(tc.raw), nil
				}
				if joined != "list table ip wgsimple_internet" {
					t.Fatal("Unexpected or mutating command", name, args)
				}
				if tc.queryFails {
					return nil, errors.New("read failed")
				}
				return []byte(tc.text), nil
			}
			normalized, err := readFirewallRules(context.Background(), run)
			found := false
			if err == nil {
				found, err = inspectManagedInternetRules(normalized, "none")
			}
			if (err != nil) != tc.wantError || err == nil && found != tc.wantOwned || calls != tc.calls {
				t.Fatal(found, err, calls)
			}
		})
	}
}
