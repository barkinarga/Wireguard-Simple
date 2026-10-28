package app

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
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
