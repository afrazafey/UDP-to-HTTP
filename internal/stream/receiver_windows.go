//go:build windows

package stream

import (
	"encoding/binary"
	"golang.org/x/sys/windows"
	"net"
	"unsafe"
)

func (r *udpReceiver) configureMulticast() error {
	// Winsock's loopback flag controls receiving locally originated multicast,
	// unlike POSIX's sender flag. ListenMulticastUDP disables it by default.
	if err := r.packet.SetMulticastLoopback(true); err != nil {
		return err
	}
	raw, err := r.conn.SyscallConn()
	if err != nil {
		return err
	}
	var socketErr error
	if err = raw.Control(func(fd uintptr) {
		socketErr = windows.SetsockoptInt(windows.Handle(fd), windows.IPPROTO_IP, windows.IP_PKTINFO, 1)
	}); err != nil {
		return err
	}
	return socketErr
}

// Go's ReadMsgUDP uses WSARecvMsg and its network poller, so cancellation/read
// deadlines remain effective. No blocking raw Winsock receive is introduced.
func (r *udpReceiver) readMulticast(buffer []byte) (int, bool, error) {
	var oob [128]byte
	n, oobn, flags, _, err := r.conn.ReadMsgUDP(buffer, oob[:])
	if err != nil {
		return n, false, err
	}
	if flags&windows.MSG_CTRUNC != 0 {
		return n, false, nil
	}
	destination, index, ok := windowsPacketInfo(oob[:oobn])
	return n, ok && destination.Equal(r.group) && index == r.iface.Index, nil
}
func windowsPacketInfo(oob []byte) (net.IP, int, bool) {
	word := int(unsafe.Sizeof(uintptr(0)))
	header := int(unsafe.Sizeof(windows.WSACMSGHDR{}))
	for len(oob) >= header {
		length := uint64(binary.LittleEndian.Uint32(oob))
		if word == 8 {
			length = binary.LittleEndian.Uint64(oob)
		}
		if length < uint64(header) || length > uint64(len(oob)) {
			return nil, 0, false
		}
		level := binary.LittleEndian.Uint32(oob[word:])
		typ := binary.LittleEndian.Uint32(oob[word+4:])
		if level == windows.IPPROTO_IP && typ == windows.IP_PKTINFO && length >= uint64(header+8) {
			data := oob[header:]
			return net.IPv4(data[0], data[1], data[2], data[3]), int(binary.LittleEndian.Uint32(data[4:])), true
		}
		next := (int(length) + word - 1) &^ (word - 1)
		if next > len(oob) {
			return nil, 0, false
		}
		oob = oob[next:]
	}
	return nil, 0, false
}
