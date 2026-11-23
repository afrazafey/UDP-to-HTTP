//go:build linux || windows

package stream

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/udphttp-proxy/udphttp-proxy/internal/network"
	"golang.org/x/net/ipv4"
)

func TestMulticastJoinForwardLeave(t *testing.T) {
	items, err := network.Discover()
	if err != nil {
		t.Fatal(err)
	}
	var name string
	for _, item := range items {
		if item.Up && item.Multicast && !item.Loopback && len(item.IPv4) > 0 {
			name = item.Name
			break
		}
	}
	if name == "" {
		t.Skip("host has no up multicast-capable IPv4 interface")
	}
	o := testOptions()
	o.Policy.AllowedMulticastGroups = []string{"239.192.0.77"}
	m, err := NewManager(context.Background(), o)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	c := Channel{ID: "multicast", Name: "Multicast test", SourceType: "multicast", SourceAddress: "239.192.0.77", SourcePort: freeUDP(t), Interface: name, Enabled: true}
	if err = m.Put(context.Background(), c, true); err != nil {
		t.Fatal(err)
	}
	sub, err := m.Subscribe(c.ID, "local")
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Close()
	waitReceiver(t, m, c.ID)
	iface, err := net.InterfaceByName(name)
	if err != nil {
		t.Fatal(err)
	}
	conn, err := net.ListenPacket("udp4", "0.0.0.0:0")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	sender := ipv4.NewPacketConn(conn)
	if err = sender.SetMulticastInterface(iface); err != nil {
		t.Fatal(err)
	}
	if err = sender.SetMulticastLoopback(true); err != nil {
		t.Fatal(err)
	}
	if err = sender.SetMulticastTTL(1); err != nil {
		t.Fatal(err)
	}
	want := tsPackets(7, 0)
	if _, err = sender.WriteTo(want, nil, &net.UDPAddr{IP: net.ParseIP(c.SourceAddress), Port: c.SourcePort}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	got, err := sub.Next(ctx)
	if err != nil || string(got) != string(want) {
		snapshot, _ := m.Get(c.ID)
		t.Fatalf("multicast forwarding failed on %s: %v; socket=%v membership=%v datagrams=%d rejected=%d errors=%v", name, err, snapshot.SocketBound, snapshot.MembershipJoined, snapshot.Counters.Datagrams, snapshot.Counters.RejectedDatagrams, snapshot.Errors)
	}
	if err = m.Action(c.ID, "stop"); err != nil {
		t.Fatal(err)
	}
	r, err := openReceiver(c.Defaults(100, 256<<10, 4<<20))
	if err != nil {
		t.Fatal("rejoin after cleanup", err)
	}
	r.Close()
}
