package stream

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"time"

	"golang.org/x/net/ipv4"
)

type udpReceiver struct {
	conn      *net.UDPConn
	packet    *ipv4.PacketConn
	group     net.IP
	iface     *net.Interface
	bind      netip.Addr
	sender    netip.Addr
	selection string
}

func prepareReceiver(c Channel) (*udpReceiver, error) {
	selected, bind, err := resolveInterface(c.Interface, c.SourceType == "multicast")
	if err != nil {
		return nil, fmt.Errorf("select UDP interface: %w", err)
	}
	iface, err := net.InterfaceByIndex(selected.Index)
	if err != nil {
		return nil, err
	}
	r := &udpReceiver{iface: iface, bind: bind, selection: c.Interface, sender: netip.MustParseAddr(c.SourceAddress)}
	if c.SourceType == "multicast" {
		r.group = net.ParseIP(c.SourceAddress)
	}
	return r, nil
}
func (r *udpReceiver) identity(c Channel) string {
	if c.SourceType == "multicast" {
		return fmt.Sprintf("multicast/%s:%d/if%d", c.SourceAddress, c.SourcePort, r.iface.Index)
	}
	return fmt.Sprintf("unicast/%s:%d/if%d", r.bind, c.SourcePort, r.iface.Index)
}
func openReceiver(c Channel) (*udpReceiver, error) {
	r, err := prepareReceiver(c)
	if err != nil {
		return nil, err
	}
	if err = r.open(c); err != nil {
		return nil, err
	}
	return r, nil
}
func (r *udpReceiver) open(c Channel) error {
	var err error
	if c.SourceType == "multicast" {
		r.conn, err = net.ListenMulticastUDP("udp4", r.iface, &net.UDPAddr{IP: r.group, Port: c.SourcePort})
		if err == nil {
			r.packet = ipv4.NewPacketConn(r.conn)
			err = r.configureMulticast()
		}
	} else {
		r.conn, err = net.ListenUDP("udp4", &net.UDPAddr{IP: net.IP(r.bind.AsSlice()), Port: c.SourcePort})
	}
	if err == nil {
		err = r.conn.SetReadBuffer(c.UDPReceiveBuffer)
	}
	if err != nil {
		r.Close()
		return fmt.Errorf("open %s UDP %s:%d on %s: %w", c.SourceType, c.SourceAddress, c.SourcePort, c.Interface, err)
	}
	return nil
}
func (r *udpReceiver) Close() {
	if r.packet != nil && r.group != nil {
		_ = r.packet.LeaveGroup(r.iface, &net.UDPAddr{IP: r.group})
	}
	if r.conn != nil {
		_ = r.conn.Close()
	}
}
func (r *udpReceiver) Read(ctx context.Context, buffer []byte) (int, bool, error) {
	if err := ctx.Err(); err != nil {
		return 0, false, err
	}
	if err := r.conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond)); err != nil {
		return 0, false, err
	}
	if r.group != nil {
		return r.readMulticast(buffer)
	}
	n, from, err := r.conn.ReadFromUDPAddrPort(buffer)
	return n, err == nil && from.Addr().Unmap() == r.sender, err
}
func (r *udpReceiver) CheckInterface() error {
	item, bind, err := resolveInterface(r.selection, r.group != nil)
	if err != nil {
		return err
	}
	if item.Index != r.iface.Index || bind != r.bind {
		return fmt.Errorf("interface %s changed index or IPv4 address; reopening UDP socket", r.selection)
	}
	return nil
}
