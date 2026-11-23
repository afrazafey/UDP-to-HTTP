//go:build windows

package stream

import (
	"encoding/binary"
	"golang.org/x/sys/windows"
	"net"
	"testing"
	"unsafe"
)

func TestWindowsPacketInfoValidation(t *testing.T) {
	h := int(unsafe.Sizeof(windows.WSACMSGHDR{}))
	word := int(unsafe.Sizeof(uintptr(0)))
	data := make([]byte, h+8)
	binary.LittleEndian.PutUint32(data, uint32(len(data)))
	binary.LittleEndian.PutUint32(data[word:], windows.IPPROTO_IP)
	binary.LittleEndian.PutUint32(data[word+4:], windows.IP_PKTINFO)
	copy(data[h:], []byte{239, 192, 0, 77})
	binary.LittleEndian.PutUint32(data[h+4:], 7)
	ip, index, ok := windowsPacketInfo(data)
	if !ok || !ip.Equal(net.IPv4(239, 192, 0, 77)) || index != 7 {
		t.Fatal(ip, index, ok)
	}
	for _, bad := range [][]byte{nil, data[:h-1], data[:h+3], append([]byte{255}, data[1:]...)} {
		if _, _, ok := windowsPacketInfo(bad); ok {
			t.Fatal("malformed packet info accepted")
		}
	}
}
