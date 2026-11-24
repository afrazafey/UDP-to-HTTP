package stream

import "bytes"

const PacketSize = 188

type pidState struct {
	seen   bool
	cc     byte
	packet [PacketSize]byte
}

// Framer keeps at most two packets while searching for three sync bytes, plus an
// incomplete packet. PID history is bounded by the MPEG-TS 13-bit PID space.
type Framer struct {
	pending []byte
	synced  bool
	pids    [8192]*pidState
}
type FrameStats struct{ Packets, CorruptPackets, DiscardedBytes, ContinuityErrors, Discontinuities, DuplicatePackets, SyncErrors uint64 }

func (f *Framer) Push(data []byte) ([]byte, FrameStats) {
	f.pending = append(f.pending, data...)
	var stats FrameStats
	var output []byte
	for {
		if !f.synced {
			found := -1
			for i := 0; i+2*PacketSize < len(f.pending); i++ {
				if f.pending[i] == 0x47 && f.pending[i+PacketSize] == 0x47 && f.pending[i+2*PacketSize] == 0x47 {
					found = i
					break
				}
			}
			if found < 0 {
				discard := len(f.pending) - 2*PacketSize
				if discard > 0 {
					stats.DiscardedBytes += uint64(discard)
					f.pending = append([]byte(nil), f.pending[discard:]...)
				}
				break
			}
			stats.DiscardedBytes += uint64(found)
			f.pending = f.pending[found:]
			f.synced = true
		}
		if len(f.pending) < PacketSize {
			break
		}
		if f.pending[0] != 0x47 {
			stats.SyncErrors++
			stats.CorruptPackets++
			f.synced = false
			continue
		}
		packet := f.pending[:PacketSize]
		f.pending = f.pending[PacketSize:]
		afc := (packet[3] >> 4) & 3
		if afc == 0 || (afc == 2 && packet[4] != 183) || (afc == 3 && packet[4] > 182) {
			stats.CorruptPackets++
			stats.DiscardedBytes += PacketSize
			continue
		}
		stats.Packets++
		pid := int(packet[1]&0x1f)<<8 | int(packet[2])
		state := f.pids[pid]
		if state == nil {
			state = &pidState{}
			f.pids[pid] = state
		}
		tei := packet[1]&0x80 != 0
		discontinuity := (afc == 2 || afc == 3) && packet[4] > 0 && packet[5]&0x80 != 0
		if discontinuity {
			stats.Discontinuities++
			state.seen = false
		}
		if tei {
			stats.CorruptPackets++
			state.seen = false
		}
		if pid != 0x1fff && !tei && !discontinuity && afc == 2 && state.seen && packet[3]&15 != state.cc {
			stats.ContinuityErrors++
		}
		if pid != 0x1fff && !tei && (afc == 1 || afc == 3) {
			cc := packet[3] & 15
			if state.seen {
				if cc == state.cc && bytes.Equal(packet, state.packet[:]) {
					stats.DuplicatePackets++
				} else if cc != (state.cc+1)&15 {
					stats.ContinuityErrors++
				}
			}
			state.seen = true
			state.cc = cc
			copy(state.packet[:], packet)
		}
		output = append(output, packet...)
	}
	// Detach from potentially large UDP buffers without retaining their backing array.
	if len(f.pending) == 0 {
		f.pending = nil
	} else {
		f.pending = append([]byte(nil), f.pending...)
	}
	return output, stats
}
