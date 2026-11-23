package network

import (
	"errors"
	"net"
	"reflect"
	"testing"
)

func TestDiscover(t *testing.T) {
	items, err := discover(func() ([]net.Interface, error) {
		return []net.Interface{{Index: 2, Name: "eth0", MTU: 1500, Flags: net.FlagUp | net.FlagMulticast}, {Index: 1, Name: "lo", Flags: net.FlagUp | net.FlagLoopback}}, nil
	}, func(i net.Interface) ([]net.Addr, error) {
		if i.Name == "lo" {
			return []net.Addr{&net.IPNet{IP: net.ParseIP("127.0.0.1"), Mask: net.CIDRMask(8, 32)}}, nil
		}
		return []net.Addr{&net.IPNet{IP: net.ParseIP("10.14.0.5"), Mask: net.CIDRMask(24, 32)}, &net.IPNet{IP: net.ParseIP("fe80::1"), Mask: net.CIDRMask(64, 128)}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].Name != "lo" || !items[0].Loopback || !items[1].Multicast || !reflect.DeepEqual(items[1].IPv4, []string{"10.14.0.5"}) {
		t.Fatalf("unexpected interfaces: %+v", items)
	}
}
func TestDiscoveryErrors(t *testing.T) {
	boom := errors.New("boom")
	if _, err := discover(func() ([]net.Interface, error) { return nil, boom }, nil); !errors.Is(err, boom) {
		t.Fatal(err)
	}
	if _, err := discover(func() ([]net.Interface, error) { return []net.Interface{{Name: "eth0"}}, nil }, func(net.Interface) ([]net.Addr, error) { return nil, boom }); !errors.Is(err, boom) {
		t.Fatal(err)
	}
}
func TestResolve(t *testing.T) {
	items := []Interface{{Name: "eth0", Up: true, Multicast: true, IPv4: []string{"10.14.0.5"}}, {Name: "lo", Up: true, IPv4: []string{"127.0.0.1"}}, {Name: "eth1", Up: false, IPv4: []string{"10.1.1.1"}}}
	for _, value := range []string{"eth0", "10.14.0.5"} {
		if _, err := Resolve(value, items, true); err != nil {
			t.Fatal(err)
		}
	}
	for _, value := range []string{"lo", "eth1", "missing", "10.0.0.99"} {
		if _, err := Resolve(value, items, true); err == nil {
			t.Fatalf("accepted %s", value)
		}
	}
	if v, err := Resolve("", items, true); v != nil || err != nil {
		t.Fatal("empty selection should be allowed")
	}
}
func TestDiscoverHost(t *testing.T) {
	items, err := Discover()
	if err != nil {
		t.Fatal(err)
	}
	for _, i := range items {
		if i.Name == "" || i.Index <= 0 {
			t.Fatalf("invalid interface: %+v", i)
		}
	}
}
