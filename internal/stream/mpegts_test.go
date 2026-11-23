package stream

import (
	"bytes"
	"testing"
)

func tsPackets(count int, first byte) []byte {
	data := make([]byte, count*PacketSize)
	for i := 0; i < count; i++ {
		p := data[i*PacketSize : (i+1)*PacketSize]
		for j := range p {
			p[j] = 0xff
		}
		p[0] = 0x47
		p[1] = 1
		p[2] = 0
		p[3] = 0x10 | ((first + byte(i)) & 15)
	}
	return data
}
func TestFramingAcrossDatagrams(t *testing.T) {
	want := tsPackets(21, 0)
	input := append([]byte("garbage"), want...)
	var f Framer
	var got []byte
	var discarded uint64
	for len(input) > 0 {
		n := 113
		if n > len(input) {
			n = len(input)
		}
		data, stats := f.Push(input[:n])
		got = append(got, data...)
		discarded += stats.DiscardedBytes
		input = input[n:]
	}
	if !bytes.Equal(got, want) || discarded != 7 {
		t.Fatalf("forwarded %d/%d bytes; discarded %d", len(got), len(want), discarded)
	}
}
func TestCorruptAndContinuity(t *testing.T) {
	var f Framer
	data := tsPackets(7, 0)
	// A missing CC, a transport-error flag, and an invalid adaptation control.
	data[3*188+3] = 0x15
	data[4*188+1] |= 0x80
	data[5*188+3] = 0
	output, stats := f.Push(data)
	if stats.Packets != 6 || stats.CorruptPackets != 2 || stats.ContinuityErrors != 1 || stats.DiscardedBytes != 188 || len(output) != 6*188 {
		t.Fatalf("%+v bytes=%d", stats, len(output))
	}
	if output[4*188+1]&0x80 == 0 {
		t.Fatal("TEI packet was rewritten")
	}
}
func TestDuplicateAndDiscontinuity(t *testing.T) {
	var f Framer
	initial := tsPackets(3, 0)
	f.Push(initial)
	duplicate := initial[2*188:]
	_, s := f.Push(duplicate)
	if s.DuplicatePackets != 1 || s.ContinuityErrors != 0 {
		t.Fatal(s)
	}
	p := tsPackets(1, 9)
	p[3] = 0x39
	p[4] = 1
	p[5] = 0x80
	_, s = f.Push(p)
	if s.Discontinuities != 1 || s.ContinuityErrors != 0 {
		t.Fatal(s)
	}
	_, s = f.Push(tsPackets(1, 10))
	if s.ContinuityErrors != 0 {
		t.Fatal(s)
	}
}
func TestResynchronizationBounded(t *testing.T) {
	var f Framer
	f.Push(tsPackets(3, 0))
	bad := bytes.Repeat([]byte{0}, 65535)
	_, s := f.Push(bad)
	if s.CorruptPackets == 0 || len(f.pending) > 2*PacketSize {
		t.Fatalf("unbounded or undetected corruption: %+v pending=%d", s, len(f.pending))
	}
	want := tsPackets(7, 0)
	got, _ := f.Push(want)
	if !bytes.Equal(got, want) {
		t.Fatal("failed to recover synchronization")
	}
}
func FuzzFramer(f *testing.F) {
	f.Add(tsPackets(7, 0))
	f.Add([]byte{0x47, 0, 1, 0})
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 1<<20 {
			t.Skip()
		}
		var parser Framer
		for len(data) > 0 {
			n := 1316
			if n > len(data) {
				n = len(data)
			}
			output, _ := parser.Push(data[:n])
			if len(output)%188 != 0 || len(parser.pending) > 376 {
				t.Fatal("invalid framing bounds")
			}
			data = data[n:]
		}
	})
}
