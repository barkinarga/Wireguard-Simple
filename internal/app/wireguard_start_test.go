package app

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestManagedStartAndPartialFailureCleanup(t *testing.T) {
	s := DefaultServer("wg0")
	for fail := 0; fail <= 4; fail++ {
		var calls []string
		count := 0
		err := startManagedInterface(context.Background(), s, nil, func(_ context.Context, name string, args []string, input string) ([]byte, error) {
			count++
			calls = append(calls, name+" "+strings.Join(args, " "))
			if name == "wg" && (!strings.Contains(input, "PrivateKey = ") || strings.Contains(input, "Address = ")) {
				t.Fatal("invalid native wg config")
			}
			if count == fail {
				return nil, errors.New("test failure")
			}
			return nil, nil
		})
		if fail == 0 {
			want := []string{"ip link add dev wg0 type wireguard", "wg setconf wg0 /dev/stdin", "ip address add 10.64.0.1/24 dev wg0", "ip link set mtu 1420 up dev wg0"}
			if err != nil || !reflect.DeepEqual(calls, want) {
				t.Fatal(calls, err)
			}
		} else {
			if err == nil {
				t.Fatal("failure swallowed")
			}
			if fail == 1 && len(calls) != 1 {
				t.Fatal("must not delete an interface we did not create")
			}
			if fail > 1 && calls[len(calls)-1] != "ip link delete dev wg0" {
				t.Fatal("missing cleanup", calls)
			}
		}
	}
}

func TestManagedStartOmitsExpiredPeers(t *testing.T) {
	s := DefaultServer("wg0")
	past := time.Now().Add(-time.Hour)
	future := time.Now().Add(time.Hour)
	expiresDuringStart := future
	peers := []Peer{
		{Enabled: true, Address: "192.168.0.0/24", PublicKey: "expired", ExpiresAt: &past},
		{Enabled: true, Address: "10.64.0.2/32", PublicKey: "active", ExpiresAt: &future},
		{Enabled: false, Address: "10.64.0.3/32", PublicKey: "disabled"},
		{Enabled: true, Address: "10.64.0.4/32", PublicKey: "expires-during-start", ExpiresAt: &expiresDuringStart},
	}
	var config string
	err := startManagedInterface(context.Background(), s, peers, func(_ context.Context, name string, args []string, input string) ([]byte, error) {
		if name == "ip" && args[0] == "link" && args[1] == "add" {
			expiresDuringStart = past
		}
		if name == "wg" {
			config = input
		}
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(config, "PublicKey = active\n") || strings.Contains(config, "PublicKey = expired\n") || strings.Contains(config, "PublicKey = disabled\n") || strings.Contains(config, "PublicKey = expires-during-start\n") {
		t.Fatal("managed start installed the wrong peer set")
	}
}

func TestStoppedConfigMatchesAcrossExpiryWindows(t *testing.T) {
	s := DefaultServer("wg0")
	now := time.Now()
	first := now.Add(-2 * time.Hour)
	second := now.Add(-time.Hour)
	peers := []Peer{
		{Enabled: true, Address: "10.64.0.2/32", PublicKey: "first", ExpiresAt: &first},
		{Enabled: true, Address: "10.64.0.3/32", PublicKey: "second", ExpiresAt: &second},
		{Enabled: true, Address: "10.64.0.4/32", PublicKey: "active"},
	}
	for _, saved := range [][]Peer{peers, peers[1:], peers[2:]} {
		if !matchesStoppedConfig(serverConfig(s, saved, true), s, peers, now) {
			t.Errorf("canonical config with %d peers was rejected", len(saved))
		}
	}
	for _, raw := range []string{
		serverConfig(s, peers[:2], true),
		strings.Replace(serverConfig(s, peers, true), "10.64.0.4/32", "10.64.0.5/32", 1),
	} {
		if matchesStoppedConfig(raw, s, peers, now) {
			t.Error("stopped configuration with unrelated drift was accepted")
		}
	}
}
func TestManagedStartDoesNotSilentlyOmitRoutedSubnets(t *testing.T) {
	s := DefaultServer("wg0")
	err := startManagedInterface(context.Background(), s, []Peer{{Enabled: true, Address: "192.168.0.0/24"}}, func(context.Context, string, []string, string) ([]byte, error) {
		t.Fatal("network changed before validation")
		return nil, nil
	})
	if err == nil {
		t.Fatal("routed fresh start must require explicit external setup")
	}
}
