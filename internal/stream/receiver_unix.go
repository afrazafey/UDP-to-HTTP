//go:build !windows

package stream

import "golang.org/x/net/ipv4"

func (r *udpReceiver) configureMulticast() error {
	return r.packet.SetControlMessage(ipv4.FlagDst|ipv4.FlagInterface, true)
}
func (r *udpReceiver) readMulticast(buffer []byte) (int, bool, error) {
	n, control, _, err := r.packet.ReadFrom(buffer)
	return n, err == nil && control != nil && control.Dst.Equal(r.group) && control.IfIndex == r.iface.Index, err
}
