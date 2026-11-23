package stream

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"sync"
	"testing"
	"time"
)

func testOptions() Options {
	return Options{Policy: Policy{AllowedUnicastSources: []string{"127.0.0.1"}, AllowLoopbackUnicast: true}, Logger: slog.New(slog.NewJSONHandler(io.Discard, nil)), InactivityTimeout: 100 * time.Millisecond}
}
func freeUDP(t *testing.T) int {
	t.Helper()
	c, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	port := c.LocalAddr().(*net.UDPAddr).Port
	c.Close()
	return port
}
func testChannel(t *testing.T) Channel {
	return Channel{ID: "test", Name: "Test stream", SourceType: "unicast", SourceAddress: "127.0.0.1", SourcePort: freeUDP(t), Interface: "127.0.0.1", Enabled: true, Startup: "on_demand"}
}
func testManager(t *testing.T) (*Manager, Channel) {
	t.Helper()
	m, err := NewManager(context.Background(), testOptions())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(m.Close)
	c := testChannel(t)
	if err = m.Put(context.Background(), c, true); err != nil {
		t.Fatal(err)
	}
	return m, c
}
func eventually(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition not met before deadline")
}
func sendUDP(t *testing.T, port int, data []byte) {
	t.Helper()
	conn, err := net.DialUDP("udp4", nil, &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: port})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err = conn.Write(data); err != nil {
		t.Fatal(err)
	}
}
func waitReceiver(t *testing.T, m *Manager, id string) {
	t.Helper()
	eventually(t, func() bool { s, _ := m.Get(id); return s.State == "waiting" || s.State == "running" })
}
func TestUDPUnicastSharedFanout(t *testing.T) {
	m, c := testManager(t)
	a, err := m.Subscribe(c.ID, "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	b, err := m.Subscribe(c.ID, "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	waitReceiver(t, m, c.ID)
	want := tsPackets(7, 0)
	sendUDP(t, c.SourcePort, want)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	for _, sub := range []*Subscription{a, b} {
		got, err := sub.Next(ctx)
		if err != nil || string(got) != string(want) {
			t.Fatalf("payload mismatch %d: %v", len(got), err)
		}
		sub.RecordOutput(len(got))
	}
	s, _ := m.Get(c.ID)
	if s.Counters.InputBytes != uint64(len(want)) || s.Counters.OutputBytes != uint64(2*len(want)) || s.Counters.Datagrams != 1 || s.Clients != 2 {
		t.Fatalf("upstream duplicated or counters wrong: %+v", s)
	}
}
func TestLifecycleAndSocketCleanup(t *testing.T) {
	m, c := testManager(t)
	for _, action := range []string{"start", "restart", "stop", "start", "stop"} {
		if err := m.Action(c.ID, action); err != nil {
			t.Fatal(err)
		}
		if action != "stop" {
			waitReceiver(t, m, c.ID)
		}
	}
	if _, err := m.Subscribe(c.ID, "local"); !errors.Is(err, ErrDisabled) {
		t.Fatal("manual stop did not block on-demand playback")
	}
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: c.SourcePort})
	if err != nil {
		t.Fatalf("receiver leaked socket: %v", err)
	}
	conn.Close()
}
func TestInactivityAndIdleRelease(t *testing.T) {
	m, c := testManager(t)
	sub, err := m.Subscribe(c.ID, "local")
	if err != nil {
		t.Fatal(err)
	}
	waitReceiver(t, m, c.ID)
	eventually(t, func() bool {
		s, _ := m.Get(c.ID)
		return s.State == "waiting" && len(s.Errors) > 0 && s.Errors[len(s.Errors)-1].Operation == "inactivity"
	})
	sendUDP(t, c.SourcePort, tsPackets(7, 0))
	eventually(t, func() bool { s, _ := m.Get(c.ID); return s.State == "running" })
	sub.Close()
	eventually(t, func() bool { s, _ := m.Get(c.ID); return s.State == "stopped" })
}
func TestClientLimitAndCancellation(t *testing.T) {
	o := testOptions()
	o.MaxClients = 1
	m, err := NewManager(context.Background(), o)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	c := testChannel(t)
	if err = m.Put(context.Background(), c, true); err != nil {
		t.Fatal(err)
	}
	sub, err := m.Subscribe(c.ID, "local")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = m.Subscribe(c.ID, "other"); !errors.Is(err, ErrLimit) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = sub.Next(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	sub.Close()
	if len(m.Clients()) != 0 {
		t.Fatal("client leaked")
	}
}
func TestSlowClientIsolationAndBudgets(t *testing.T) {
	m, err := NewManager(context.Background(), testOptions())
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	e := newEntry(Channel{ID: "queue", Enabled: true, ClientBufferBytes: 3 * 188})
	m.mu.Lock()
	m.entries["queue"] = e
	newSub := func(id uint64) *Subscription {
		s := &Subscription{manager: m, entry: e, id: id, queue: make(chan []byte, 64), done: make(chan struct{})}
		e.clients[id] = s
		m.totalClients++
		return s
	}
	slow := newSub(1)
	fast := newSub(2)
	m.publishLocked(e, tsPackets(3, 0))
	m.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err = fast.Next(ctx); err != nil {
		t.Fatal(err)
	}
	m.mu.Lock()
	m.publishLocked(e, tsPackets(3, 3))
	queued := m.totalQueued
	m.mu.Unlock()
	select {
	case <-slow.Done():
	default:
		t.Fatal("slow client not disconnected")
	}
	if _, err = fast.Next(ctx); err != nil {
		t.Fatal("fast client disconnected", err)
	}
	if queued != 3*188 {
		t.Fatalf("queue accounting %d", queued)
	}
	slow.Close()
	fast.Close()
	if m.totalQueued != 0 || m.totalClients != 0 {
		t.Fatal("queue budget leaked")
	}
}
func TestConcurrentSubscribeUnsubscribeAndActions(t *testing.T) {
	m, c := testManager(t)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 5; j++ {
				sub, err := m.Subscribe(c.ID, "local")
				if err == nil {
					sub.Close()
				}
				m.List()
				m.Clients()
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 3; i++ {
			_ = m.Action(c.ID, "restart")
			_ = m.Action(c.ID, "stop")
			_ = m.Action(c.ID, "start")
		}
	}()
	wg.Wait()
	m.Close()
	if m.totalQueued != 0 || m.totalClients != 0 {
		t.Fatal("resources leaked after client churn")
	}
}
func TestDuplicateSourcesAndValidation(t *testing.T) {
	m, c := testManager(t)
	c.ID = "duplicate"
	if err := m.Put(context.Background(), c, true); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	c.ID = "unsafe"
	c.SourceAddress = "169.254.169.254"
	if err := m.Put(context.Background(), c, true); err == nil {
		t.Fatal("unapproved source accepted")
	}
	c.SourceAddress = "localhost"
	if err := m.Put(context.Background(), c, true); err == nil {
		t.Fatal("hostname accepted")
	}
}
func TestRetryAndShutdown(t *testing.T) {
	m, c := testManager(t)
	occupied, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: c.SourcePort})
	if err != nil {
		t.Fatal(err)
	}
	if err = m.Action(c.ID, "start"); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool { s, _ := m.Get(c.ID); return s.Counters.Retries > 0 && s.State == "retrying" })
	occupied.Close()
	waitReceiver(t, m, c.ID)
	m.Close()
	s, _ := m.Get(c.ID)
	if s.State != "stopped" {
		t.Fatal(s.State)
	}
}

func TestUnicastRejectsUnexpectedSender(t *testing.T) {
	m, c := testManager(t)
	sub, err := m.Subscribe(c.ID, "local")
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Close()
	waitReceiver(t, m, c.ID)
	other, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 2)})
	if err != nil {
		t.Skipf("host does not support binding a second loopback sender: %v", err)
	}
	defer other.Close()
	if _, err = other.WriteToUDP(tsPackets(7, 0), &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: c.SourcePort}); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool { s, _ := m.Get(c.ID); return s.Counters.RejectedDatagrams == 1 })
	s, _ := m.Get(c.ID)
	if s.Counters.InputBytes != 0 || s.Counters.Packets != 0 {
		t.Fatal("unexpected source entered the stream", s.Counters)
	}
	sendUDP(t, c.SourcePort, tsPackets(7, 0))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err = sub.Next(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestResolvedReceiverReservation(t *testing.T) {
	m, c := testManager(t)
	if err := m.Action(c.ID, "start"); err != nil {
		t.Fatal(err)
	}
	waitReceiver(t, m, c.ID)
	// Simulate two persisted selections converging after an interface change.
	other := c.Defaults(100, 256<<10, 4<<20)
	other.ID = "converged"
	m.mu.Lock()
	m.entries[other.ID] = newEntry(other)
	m.mu.Unlock()
	if err := m.Action(other.ID, "start"); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() bool { s, _ := m.Get(other.ID); return s.State == "retrying" && s.Counters.Retries > 0 })
	m.mu.Lock()
	count := len(m.receivers)
	m.mu.Unlock()
	if count != 1 {
		t.Fatalf("opened duplicate receiver: %d", count)
	}
	if err := m.Action(c.ID, "stop"); err != nil {
		t.Fatal(err)
	}
	waitReceiver(t, m, other.ID)
	m.Close()
	if len(m.receivers) != 0 {
		t.Fatal("receiver reservation leaked")
	}
}
func TestClientMeasurementsAndDisconnect(t *testing.T) {
	m, c := testManager(t)
	sub, err := m.Subscribe(c.ID, "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Close()
	sub.RecordOutput(1880)
	clients := m.Clients()
	if len(clients) != 1 || clients[0].BytesDelivered != 1880 || clients[0].OutputBitrate <= 0 || clients[0].DurationSeconds < 0 {
		t.Fatal("invalid client measurements", clients)
	}
	if err = m.Disconnect(clients[0].ID); err != nil {
		t.Fatal(err)
	}
	select {
	case <-sub.Done():
	default:
		t.Fatal("disconnect did not close subscription")
	}
	clients = m.Clients()
	if len(clients) != 1 || !clients[0].Closing {
		t.Fatal("closing state not exposed", clients)
	}
	sub.Close()
	if len(m.Clients()) != 0 {
		t.Fatal("client slot not released")
	}
}
