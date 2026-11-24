package netpolicy

import (
	"fmt"
	"net/netip"
	"slices"
	"strings"
	"testing"
)

func TestClientNormalization(t *testing.T) {
	p, err := Compile("clients", List{" 10.14.0.5\r\n192.168.1.81/24, 10.14.0.5/32 ", "", "2001:0db8:0::12/64", "::ffff:10.14.0.5"}, Client, false)
	if err != nil {
		t.Fatal(err)
	}
	if got := p.Values(); !slices.Equal(got, List{"10.14.0.5", "192.168.1.0/24", "2001:db8::/64"}) {
		t.Fatal(got)
	}
	for _, addr := range []string{"::ffff:10.14.0.5", "10.14.0.5", "192.168.1.30", "2001:db8::2"} {
		if !p.Check(netip.MustParseAddr(addr)).Allowed {
			t.Errorf("missed %s", addr)
		}
	}
	for _, addr := range []string{"10.14.0.6", "2001:db8:1::2", "fe80::1%eth0"} {
		if p.Check(netip.MustParseAddr(addr)).Allowed {
			t.Errorf("unexpected match %s", addr)
		}
	}
	empty := Prefixes{}
	if empty.Check(netip.MustParseAddr("10.14.0.5")).Allowed {
		t.Fatal("empty allowlist matched")
	}
}

func TestInvalidPolicyEntries(t *testing.T) {
	for _, test := range []struct {
		kind  Kind
		value string
	}{
		{Client, "127.0.0.1/33"}, {Client, "fe80::1%eth0"}, {Client, "::ffff:10.0.0.5/95"},
		{MulticastAllow, "239.0.0.0/3"}, {MulticastAllow, "ff00::/8"}, {MulticastAllow, "224.0.0.7"},
		{MulticastAllow, "example.test"}, {MulticastBlock, "240.0.0.0/8"},
		{Unicast, "127.0.0.0/8"}, {Unicast, "::ffff:10.1.1.1"}, {Unicast, "999.0.0.1"},
		{Proxy, "127.0.0.1/32"}, {Proxy, "10.1.1.1"}, {Bind, "239.0.0.1"},
	} {
		if _, err := Compile("test_field", List{test.value}, test.kind, false); err == nil ||
			!strings.Contains(err.Error(), test.value) || !strings.Contains(err.Error(), "test_field") {
			t.Errorf("%s: %v", test.value, err)
		}
	}
}

func TestSourcePrecedence(t *testing.T) {
	s, err := CompileSources(List{"224.0.0.0/4"}, List{"239.20.0.0/16"}, List{"0.0.0.0/0"}, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		kind, address, match string
		allowed              bool
	}{
		{"multicast", "224.0.0.12", "224.0.0.0/24", false},
		{"multicast", "239.20.4.1", "239.20.0.0/16", false},
		{"multicast", "239.21.4.1", "224.0.0.0/4", true},
		{"unicast", "10.20.7.1", "0.0.0.0/0", true},
		{"unicast", "127.0.0.1", "", false},
		{"unicast", "169.254.1.1", "", false},
		{"unicast", "0.0.0.0", "", false},
		{"unicast", "255.255.255.255", "", false},
		{"unicast", "239.1.1.1", "", false},
	} {
		if got := s.Check(test.kind, netip.MustParseAddr(test.address)); got.Allowed != test.allowed || got.Matched != test.match {
			t.Errorf("%s: %+v", test.address, got)
		}
	}
}

func TestMulticastAllCoverage(t *testing.T) {
	for _, test := range []struct {
		ranges List
		all    bool
	}{
		{List{"239.0.0.0/8"}, false},
		{List{"224.0.0.0/4"}, true},
		{List{"232.0.0.0/5,224.0.0.0/5,239.0.0.0/8"}, true},
		{List{"224.0.0.0/5,232.0.0.0/6,238.0.0.0/7"}, false},
	} {
		sources, err := CompileSources(test.ranges, List{"224.0.0.0/24"}, nil, false)
		if err != nil || sources.AllowsAllMulticast() != test.all {
			t.Errorf("%v: all=%v, error=%v", test.ranges, sources.AllowsAllMulticast(), err)
		}
	}
}

func BenchmarkClientMatch(b *testing.B) {
	for _, size := range []int{8, 128, 1024} {
		b.Run(fmt.Sprint(size), func(b *testing.B) {
			list := make(List, 0, size)
			for i := range size {
				list = append(list, fmt.Sprintf("10.%d.%d.0/24", i/256, i%256))
			}
			p, err := Compile("clients", list, Client, false)
			if err != nil {
				b.Fatal(err)
			}
			ip := netip.MustParseAddr("192.0.2.1")
			b.ReportAllocs()
			for b.Loop() {
				p.Match(ip)
			}
		})
	}
}
