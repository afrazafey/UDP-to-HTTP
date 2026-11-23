package stream

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"
)

func TestRecoveryDelayBounded(t *testing.T) {
	p := DefaultRecovery()
	for _, base := range []time.Duration{time.Nanosecond, time.Second, time.Hour} {
		for i := 0; i < 100; i++ {
			v := p.delay(base)
			if v < p.MinDelay || v > p.MaxDelay {
				t.Fatal(v)
			}
		}
	}
}
func TestRecoveryAttemptBudgetRequiresManualRestart(t *testing.T) {
	o := testOptions()
	p := DefaultRecovery()
	p.MaxAttempts = 1
	p.MinDelay = 10 * time.Millisecond
	p.MaxDelay = 20 * time.Millisecond
	o.Recovery = &p
	m, err := NewManager(t.Context(), o)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	c := testChannel(t)
	blocker, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: c.SourcePort})
	if err != nil {
		t.Fatal(err)
	}
	defer blocker.Close()
	if err = m.Put(t.Context(), c, true); err != nil {
		t.Fatal(err)
	}
	if err = m.Action(c.ID, "start"); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool { v, _ := m.Get(c.ID); return v.State == "stopped" && v.Counters.ReceiverRestarts == 1 })
	if sub, err := m.Subscribe(c.ID, "local"); err == nil {
		sub.Close()
		t.Fatal("exhausted receiver restarted without manual intervention")
	}
	blocker.Close()
	if err = m.Action(c.ID, "start"); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool { v, _ := m.Get(c.ID); return v.SocketBound })
}
func TestRecoveryAfterInactivityPreservesSharedViewers(t *testing.T) {
	o := testOptions()
	p := DefaultRecovery()
	p.MinDelay = 10 * time.Millisecond
	p.MaxDelay = 30 * time.Millisecond
	p.RejoinAfter = 100 * time.Millisecond
	o.Recovery = &p
	m, err := NewManager(context.Background(), o)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	c := testChannel(t)
	c.IdleTimeoutSeconds = 1
	if err = m.Put(context.Background(), c, true); err != nil {
		t.Fatal(err)
	}
	a, err := m.Subscribe(c.ID, "local-a")
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	b, err := m.Subscribe(c.ID, "local-b")
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	eventually(t, func() bool { v, _ := m.Get(c.ID); return v.Counters.ReceiverRestarts > 0 && v.SocketBound })
	for i := 0; i < 8; i++ {
		sendUDP(t, c.SourcePort, tsPackets(7, 0))
		v, _ := m.Get(c.ID)
		if v.Health == "healthy" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	eventually(t, func() bool {
		v, _ := m.Get(c.ID)
		return v.Counters.Recoveries == 1 && v.LastRecoveryAt != nil && v.LastDatagramAt != nil && v.Clients == 2
	})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	for _, s := range []*Subscription{a, b} {
		if _, err = s.Next(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if m.ReceiverCount() != 1 {
		t.Fatal("duplicate receivers", m.ReceiverCount())
	}
	a.Close()
	b.Close()
	eventually(t, func() bool { v, _ := m.Get(c.ID); return v.State == "stopped" && m.ReceiverCount() == 0 })
}
func TestRecoveryAfterSocketFailure(t *testing.T) {
	o := testOptions()
	opened := make(chan *net.UDPConn, 8)
	o.socketOpened = func(c *net.UDPConn) { opened <- c }
	p := DefaultRecovery()
	p.MinDelay = 10 * time.Millisecond
	p.MaxDelay = 30 * time.Millisecond
	o.Recovery = &p
	m, err := NewManager(context.Background(), o)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	c := testChannel(t)
	if err = m.Put(context.Background(), c, true); err != nil {
		t.Fatal(err)
	}
	if err = m.Action(c.ID, "start"); err != nil {
		t.Fatal(err)
	}
	select {
	case conn := <-opened:
		conn.Close()
	case <-time.After(time.Second):
		t.Fatal("socket did not open")
	}
	eventually(t, func() bool { v, _ := m.Get(c.ID); return v.Counters.ReceiverRestarts > 0 && v.SocketBound })
	sendUDP(t, c.SourcePort, tsPackets(7, 0))
	eventually(t, func() bool { v, _ := m.Get(c.ID); return v.Health == "healthy" && v.Counters.Recoveries == 1 })
	m.Close()
	if m.ReceiverCount() != 0 {
		t.Fatal("receiver reservation leaked")
	}
}
func TestRecoveryAfterSimulatedInterfaceInterruption(t *testing.T) {
	var down atomic.Bool
	down.Store(true)
	o := testOptions()
	o.interfaceCheck = func(r *udpReceiver) error {
		if down.Load() {
			return errors.New("simulated interface disappearance")
		}
		return r.CheckInterface()
	}
	p := DefaultRecovery()
	p.MinDelay = 10 * time.Millisecond
	p.MaxDelay = 30 * time.Millisecond
	o.Recovery = &p
	m, err := NewManager(context.Background(), o)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	c := testChannel(t)
	if err = m.Put(context.Background(), c, true); err != nil {
		t.Fatal(err)
	}
	m.Action(c.ID, "start")
	eventually(t, func() bool { v, _ := m.Get(c.ID); return v.Counters.ReceiverRestarts > 0 })
	down.Store(false)
	eventually(t, func() bool { v, _ := m.Get(c.ID); return v.SocketBound })
	for i := 0; i < 10; i++ {
		sendUDP(t, c.SourcePort, tsPackets(7, 0))
		v, _ := m.Get(c.ID)
		if v.Health == "healthy" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	eventually(t, func() bool { v, _ := m.Get(c.ID); return v.Health == "healthy" })
}
func TestBatchValidationLeavesExistingChannels(t *testing.T) {
	m, c := testManager(t)
	bad := c
	bad.ID = "bad"
	bad.SourceAddress = "10.99.99.99"
	copy := c
	copy.ID = "copy"
	copy.SourcePort = freeUDP(t)
	if err := m.Batch(context.Background(), []Channel{copy, bad}, nil); err == nil {
		t.Fatal("unauthorized batch accepted")
	}
	if len(m.List()) != 1 {
		t.Fatal("partial batch applied")
	}
}
