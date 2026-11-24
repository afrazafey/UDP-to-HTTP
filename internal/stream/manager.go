package stream

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"sort"
	"sync"
	"time"

	"github.com/udphttp-proxy/udphttp-proxy/internal/netpolicy"
)

var (
	ErrNotFound     = errors.New("channel not found")
	ErrConflict     = errors.New("channel ID, source, or UDP bind conflicts with an existing channel")
	ErrLimit        = errors.New("stream or client limit reached")
	ErrDisabled     = errors.New("channel is disabled or manually stopped")
	ErrClosed       = errors.New("stream subscription closed")
	ErrStorage      = errors.New("channel persistence failed")
	ErrClientDenied = errors.New("playback client IP is not allowed")
	ErrSourceDenied = errors.New("channel source is denied by the current network policy")
)

type Repository interface {
	ListChannels(context.Context) ([]Channel, error)
	SaveChannel(context.Context, Channel, bool) error
	DeleteChannel(context.Context, string) error
}
type Options struct {
	Recovery                                                                                    *RecoveryPolicy
	interfaceCheck                                                                              func(*udpReceiver) error
	socketOpened                                                                                func(*net.UDPConn)
	Policy                                                                                      Policy
	Repository                                                                                  Repository
	Logger                                                                                      *slog.Logger
	MaxClients, MaxStreams, ClientBufferBytes, UDPReceiveBuffer, StreamBuffer, MaxBufferedBytes int
	InactivityTimeout                                                                           time.Duration
	Disabled                                                                                    bool
}
type worker struct {
	cancel   context.CancelFunc
	done     chan struct{}
	pinned   bool
	stopping bool
}
type entry struct {
	lastDatagram, lastRecovery                     time.Time
	socketBound, membershipJoined, recoveryPending bool
	channel                                        Channel
	state                                          string
	blocked                                        bool
	policyBlocked                                  bool
	worker                                         *worker
	clients                                        map[uint64]*Subscription
	queued                                         int
	started                                        time.Time
	lastPacket                                     time.Time
	idleSince                                      time.Time
	counters                                       Counters
	errors                                         []RecentError
	sampleTime                                     time.Time
	sampleIn, sampleOut                            uint64
	inRate, outRate                                float64
}
type Manager struct {
	mu                        sync.Mutex
	operations                sync.Mutex
	ctx                       context.Context
	cancel                    context.CancelFunc
	options                   Options
	sources                   netpolicy.Sources
	playbackClients           netpolicy.Prefixes
	entries                   map[string]*entry
	receivers                 map[string]*worker
	closed                    bool
	nextClient                uint64
	totalClients, totalQueued int
}

func NewManager(ctx context.Context, o Options) (*Manager, error) {
	sources, err := o.Policy.Compile()
	if err != nil {
		return nil, err
	}
	if o.Recovery == nil {
		p := DefaultRecovery()
		o.Recovery = &p
	}
	p := *o.Recovery
	if p.MinDelay < time.Millisecond || p.MaxDelay < p.MinDelay || p.RejoinAfter < time.Millisecond || !(p.Jitter >= 0 && p.Jitter <= 0.5) || p.MaxAttempts < 0 {
		return nil, errors.New("invalid recovery policy")
	}
	o.Recovery = &p
	if o.interfaceCheck == nil {
		o.interfaceCheck = func(r *udpReceiver) error { return r.CheckInterface() }
	}
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	if o.MaxClients == 0 {
		o.MaxClients = 100
	}
	if o.MaxStreams == 0 {
		o.MaxStreams = 128
	}
	if o.ClientBufferBytes == 0 {
		o.ClientBufferBytes = 256 << 10
	}
	if o.UDPReceiveBuffer == 0 {
		o.UDPReceiveBuffer = 4 << 20
	}
	if o.StreamBuffer == 0 {
		o.StreamBuffer = 8 << 20
	}
	if o.MaxBufferedBytes == 0 {
		o.MaxBufferedBytes = 64 << 20
	}
	if o.InactivityTimeout == 0 {
		o.InactivityTimeout = 10 * time.Second
	}
	ctx, cancel := context.WithCancel(ctx)
	m := &Manager{ctx: ctx, cancel: cancel, options: o, sources: sources, entries: map[string]*entry{}, receivers: map[string]*worker{}}
	if o.Repository != nil {
		channels, err := o.Repository.ListChannels(ctx)
		if err != nil {
			cancel()
			return nil, err
		}
		if len(channels) > o.MaxStreams {
			cancel()
			return nil, fmt.Errorf("stored channel count exceeds max_streams")
		}
		for _, channel := range channels {
			c := channel.Defaults(o.MaxClients, o.ClientBufferBytes, o.UDPReceiveBuffer)
			e := newEntry(c)
			m.entries[c.ID] = e
			if _, err = m.validate(c); err != nil {
				e.policyBlocked = true
				e.state = "error"
				m.errorLocked(e, "policy", err)
			}
		}
		// Re-evaluate all identities before opening any sockets. Policy changes or
		// interface changes cannot silently create duplicate upstream receivers.
		keys := map[string]bool{}
		for _, e := range m.entries {
			key := e.channel.SourceKey()
			if keys[key] {
				cancel()
				return nil, ErrConflict
			}
			keys[key] = true
		}
		m.mu.Lock()
		for _, e := range m.entries {
			if e.channel.Enabled && !e.policyBlocked && e.channel.Startup == "always" && !o.Disabled {
				m.startLocked(e, true)
			}
		}
		m.mu.Unlock()
	}
	return m, nil
}
func newEntry(c Channel) *entry {
	state := "stopped"
	if !c.Enabled {
		state = "disabled"
	}
	return &entry{channel: c, state: state, clients: map[uint64]*Subscription{}, errors: []RecentError{}, sampleTime: time.Now()}
}
func (m *Manager) validate(c Channel) (Channel, error) {
	c = c.Defaults(m.options.MaxClients, m.options.ClientBufferBytes, m.options.UDPReceiveBuffer)
	if err := c.ValidateWithSources(m.sources); err != nil {
		return c, err
	}
	if c.MaxClients > m.options.MaxClients {
		return c, errors.New("channel max_clients exceeds the global client limit")
	}
	if c.ClientBufferBytes > m.options.StreamBuffer || c.ClientBufferBytes > m.options.MaxBufferedBytes {
		return c, errors.New("client buffer exceeds stream or global buffer budget")
	}
	return c, nil
}
func (m *Manager) Put(ctx context.Context, c Channel, create bool) error {
	m.operations.Lock()
	defer m.operations.Unlock()
	c, err := m.validate(c)
	if err != nil {
		return err
	}
	if _, _, err = resolveInterface(c.Interface, c.SourceType == "multicast"); err != nil {
		return err
	}
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return ErrClosed
	}
	old, exists := m.entries[c.ID]
	if create && exists {
		m.mu.Unlock()
		return ErrConflict
	}
	if !create && !exists {
		m.mu.Unlock()
		return ErrNotFound
	}
	if create && len(m.entries) >= m.options.MaxStreams {
		m.mu.Unlock()
		return ErrLimit
	}
	key := c.SourceKey()
	for id, e := range m.entries {
		if id != c.ID && (e.channel.SourceKey() == key || unicastBindConflict(c, e.channel)) {
			m.mu.Unlock()
			return ErrConflict
		}
	}
	m.mu.Unlock()
	if m.options.Repository != nil {
		if err = m.options.Repository.SaveChannel(ctx, c, create); err != nil {
			m.options.Logger.Error("channel persistence failed", "channel_id", c.ID, "error", err)
			return fmt.Errorf("%w: %v", ErrStorage, err)
		}
	}
	if old != nil {
		m.stop(old, true)
	}
	m.mu.Lock()
	e := newEntry(c)
	m.entries[c.ID] = e
	if c.Enabled && c.Startup == "always" && !m.options.Disabled {
		m.startLocked(e, true)
	}
	m.mu.Unlock()
	return nil
}
func unicastBindConflict(a, b Channel) bool {
	if a.SourceType != "unicast" || b.SourceType != "unicast" || a.SourcePort != b.SourcePort {
		return false
	}
	ai, ab, ae := resolveInterface(a.Interface, false)
	bi, bb, be := resolveInterface(b.Interface, false)
	if ae != nil || be != nil {
		return a.Interface == b.Interface
	}
	return ai.Name == bi.Name && ab == bb
}
func (m *Manager) Delete(ctx context.Context, id string) error {
	m.operations.Lock()
	defer m.operations.Unlock()
	m.mu.Lock()
	e := m.entries[id]
	m.mu.Unlock()
	if e == nil {
		return ErrNotFound
	}
	if m.options.Repository != nil {
		if err := m.options.Repository.DeleteChannel(ctx, id); err != nil {
			m.options.Logger.Error("channel deletion failed", "channel_id", id, "error", err)
			return fmt.Errorf("%w: %v", ErrStorage, err)
		}
	}
	m.stop(e, true)
	m.mu.Lock()
	delete(m.entries, id)
	m.mu.Unlock()
	return nil
}
func (m *Manager) Action(id, action string) error {
	m.operations.Lock()
	defer m.operations.Unlock()
	m.mu.Lock()
	e := m.entries[id]
	closed := m.closed
	m.mu.Unlock()
	if closed {
		return ErrClosed
	}
	if e == nil {
		return ErrNotFound
	}
	if action != "start" && action != "stop" && action != "restart" {
		return errors.New("unknown stream action")
	}
	if action == "stop" || action == "restart" {
		m.stop(e, true)
	}
	if action == "start" || action == "restart" {
		if !e.channel.Enabled || m.options.Disabled {
			return ErrDisabled
		}
		if _, err := m.validate(e.channel); err != nil {
			return err
		}
		m.mu.Lock()
		if w := e.worker; w != nil && w.stopping {
			m.mu.Unlock()
			<-w.done
			m.mu.Lock()
		}
		e.blocked = false
		e.policyBlocked = false
		m.startLocked(e, true)
		m.mu.Unlock()
	}
	return nil
}

func (m *Manager) startLocked(e *entry, pinned bool) {
	if e.worker != nil {
		e.worker.pinned = e.worker.pinned || pinned
		return
	}
	ctx, cancel := context.WithCancel(m.ctx)
	if !e.started.IsZero() {
		e.counters.ReceiverRestarts++
	}
	w := &worker{cancel: cancel, done: make(chan struct{}), pinned: pinned}
	e.worker = w
	e.state = "starting"
	e.started = time.Now()
	e.lastPacket = time.Time{}
	e.idleSince = time.Now()
	go m.run(ctx, e, w)
}
func (m *Manager) stop(e *entry, blocked bool) {
	m.mu.Lock()
	e.blocked = blocked
	w := e.worker
	e.state = "stopped"
	if !e.channel.Enabled {
		e.state = "disabled"
	}
	for _, sub := range e.clients {
		m.terminateLocked(sub)
	}
	if w != nil {
		w.cancel()
	}
	m.mu.Unlock()
	if w != nil {
		<-w.done
	}
}
func (m *Manager) Close() {
	m.operations.Lock()
	defer m.operations.Unlock()
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return
	}
	m.closed = true
	m.cancel()
	entries := make([]*entry, 0, len(m.entries))
	for _, e := range m.entries {
		entries = append(entries, e)
	}
	m.mu.Unlock()
	for _, e := range entries {
		m.stop(e, true)
	}
}
func (m *Manager) errorLocked(e *entry, operation string, err error) {
	message := err.Error()
	if len(message) > 1024 {
		message = message[:1024]
	}
	e.errors = append(e.errors, RecentError{time.Now().UTC(), operation, message})
	if len(e.errors) > 20 {
		e.errors = append([]RecentError(nil), e.errors[len(e.errors)-20:]...)
	}
	m.options.Logger.Warn("stream event", "component", "receiver", "channel_id", e.channel.ID, "event_type", operation, "error_category", operation, "operation", operation, "error", message)
}
func (m *Manager) run(ctx context.Context, e *entry, w *worker) {
	defer func() {
		m.options.Logger.Info("receiver stopped", "component", "receiver", "channel_id", e.channel.ID, "event_type", "receiver_stop")
		m.mu.Lock()
		if e.worker == w {
			e.socketBound = false
			e.membershipJoined = false
			e.worker = nil
			e.state = "stopped"
			if e.policyBlocked {
				e.state = "error"
			} else if !e.channel.Enabled {
				e.state = "disabled"
			}
			for _, sub := range e.clients {
				m.terminateLocked(sub)
			}
		}
		m.mu.Unlock()
		close(w.done)
	}()
	policy := *m.options.Recovery
	backoff := policy.MinDelay
	attempts := 0
	for ctx.Err() == nil {
		m.mu.Lock()
		if e.policyBlocked || !e.channel.SourceDecision(m.sources).Allowed {
			m.mu.Unlock()
			return
		}
		idle := activeClients(e) == 0 && !w.pinned && time.Since(e.idleSince) >= time.Duration(e.channel.IdleTimeoutSeconds)*time.Second
		if idle {
			w.stopping = true
		}
		m.mu.Unlock()
		if idle {
			return
		}
		opened := time.Now()
		if attempts > 0 {
			m.mu.Lock()
			e.counters.ReceiverRestarts++
			m.mu.Unlock()
			m.options.Logger.Info("receiver reopening", "component", "receiver", "channel_id", e.channel.ID, "event_type", "recovery_attempt", "attempt", attempts)
		}
		r, err := prepareReceiver(e.channel)
		key := ""
		if err == nil {
			key = r.identity(e.channel)
			m.mu.Lock()
			if m.receivers[key] != nil {
				err = fmt.Errorf("%w: another channel already owns the resolved receiver", ErrConflict)
			} else {
				m.receivers[key] = w
			}
			m.mu.Unlock()
			if err == nil {
				err = r.open(e.channel)
			}
		}
		if err == nil {
			m.mu.Lock()
			e.state = "waiting"
			e.socketBound = true
			e.membershipJoined = e.channel.SourceType == "multicast"
			m.mu.Unlock()
			if m.options.socketOpened != nil {
				m.options.socketOpened(r.conn)
			}
			m.options.Logger.Info("UDP socket ready", "component", "receiver", "channel_id", e.channel.ID, "event_type", "socket_open", "multicast_joined", e.channel.SourceType == "multicast")
			err = m.receive(ctx, e, w, r)
			r.Close()
		}
		m.mu.Lock()
		e.socketBound = false
		e.membershipJoined = false
		if m.receivers[key] == w {
			delete(m.receivers, key)
		}
		m.mu.Unlock()
		if err == nil || ctx.Err() != nil {
			return
		}
		if ctx.Err() != nil {
			return
		}
		m.mu.Lock()
		if e.lastPacket.After(opened) {
			backoff = policy.MinDelay
			attempts = 0
		}
		attempts++
		e.recoveryPending = true
		e.counters.RecoveryFailures++
		e.state = "retrying"
		e.counters.Retries++
		m.errorLocked(e, "receive", err)
		wait := policy.delay(backoff)
		if activeClients(e) == 0 && !w.pinned {
			remaining := time.Duration(e.channel.IdleTimeoutSeconds)*time.Second - time.Since(e.idleSince)
			if remaining < wait {
				wait = max(0, remaining)
			}
		}
		m.mu.Unlock()
		if !policy.Enabled || (policy.MaxAttempts > 0 && attempts > policy.MaxAttempts) {
			m.mu.Lock()
			e.blocked = true
			m.mu.Unlock()
			return
		}
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		if backoff < policy.MaxDelay {
			backoff *= 2
			if backoff > policy.MaxDelay {
				backoff = policy.MaxDelay
			}
		}
	}
}
func (m *Manager) receive(ctx context.Context, e *entry, w *worker, r *udpReceiver) error {
	var framer Framer
	buffer := make([]byte, 65535)
	lastCheck := time.Now()
	inactivityReported := false
	opened := time.Now()
	for ctx.Err() == nil {
		n, accepted, err := r.Read(ctx, buffer)
		if n < 0 || n > len(buffer) {
			return fmt.Errorf("invalid UDP read length: %d", n)
		}
		now := time.Now()
		if err != nil {
			var ne net.Error
			if !errors.As(err, &ne) || !ne.Timeout() {
				return err
			}
		}
		if now.Sub(lastCheck) >= time.Second {
			if checkErr := m.options.interfaceCheck(r); checkErr != nil {
				return checkErr
			}
			lastCheck = now
		}
		var data []byte
		var stats FrameStats
		if err == nil && accepted {
			data, stats = framer.Push(buffer[:n])
		}
		m.mu.Lock()
		if e.policyBlocked {
			m.mu.Unlock()
			return context.Canceled
		}
		if err == nil {
			if !accepted {
				e.counters.RejectedDatagrams++
			} else {
				e.lastDatagram = now
				e.counters.Datagrams++
				e.counters.InputBytes += uint64(n)
				e.counters.Packets += stats.Packets
				e.counters.SyncErrors += stats.SyncErrors
				e.counters.CorruptPackets += stats.CorruptPackets
				e.counters.DiscardedBytes += stats.DiscardedBytes
				e.counters.ContinuityErrors += stats.ContinuityErrors
				e.counters.Discontinuities += stats.Discontinuities
				e.counters.DuplicatePackets += stats.DuplicatePackets
			}
		}
		if len(data) > 0 {
			if e.recoveryPending {
				e.counters.Recoveries++
				e.lastRecovery = now
				e.recoveryPending = false
				m.options.Logger.Info("valid transport recovered", "component", "receiver", "channel_id", e.channel.ID, "event_type", "recovered")
			}
			e.lastPacket = now
			e.state = "running"
			inactivityReported = false
			m.publishLocked(e, data)
		}
		last := e.lastPacket
		if last.IsZero() {
			last = opened
		}
		if now.Sub(last) >= m.options.InactivityTimeout && !inactivityReported {
			e.state = "waiting"
			m.errorLocked(e, "inactivity", fmt.Errorf("no valid MPEG-TS packets for %s; UDP socket remains open", m.options.InactivityTimeout))
			inactivityReported = true
		}
		idle := activeClients(e) == 0 && !w.pinned && now.Sub(e.idleSince) >= time.Duration(e.channel.IdleTimeoutSeconds)*time.Second
		if idle {
			w.stopping = true
		}
		lastArrival := e.lastDatagram
		m.mu.Unlock()
		if idle {
			return nil
		}
		if lastArrival.Before(opened) {
			lastArrival = opened
		}
		if m.options.Recovery.Enabled && now.Sub(lastArrival) >= m.options.Recovery.RejoinAfter {
			return errors.New("UDP source inactive; reopening socket and rejoining selected multicast interface when applicable")
		}
	}
	return ctx.Err()
}
func activeClients(e *entry) int {
	count := 0
	for _, s := range e.clients {
		if !s.closed {
			count++
		}
	}
	return count
}
func (m *Manager) publishLocked(e *entry, data []byte) {
	for _, s := range e.clients {
		if s.closed {
			continue
		}
		if s.queued+len(data) > e.channel.ClientBufferBytes || e.queued+len(data) > m.options.StreamBuffer || m.totalQueued+len(data) > m.options.MaxBufferedBytes || len(s.queue) == cap(s.queue) {
			e.counters.SlowClientDisconnects++
			m.errorLocked(e, "slow_client", fmt.Errorf("client %d exceeded a bounded queue budget and was disconnected", s.id))
			m.terminateLocked(s)
			continue
		}
		s.queue <- data
		s.queued += len(data)
		e.queued += len(data)
		m.totalQueued += len(data)
	}
}

type Subscription struct {
	manager     *Manager
	entry       *entry
	id          uint64
	queue       chan []byte
	done        chan struct{}
	queued      int
	closed      bool
	remoteIP    string
	connectedAt time.Time
	delivered   uint64
}

func (m *Manager) Subscribe(id, remoteIP string) (*Subscription, error) {
	return m.subscribe(id, remoteIP, false)
}

func (m *Manager) SubscribePlayback(id string, ip netip.Addr) (*Subscription, error) {
	return m.subscribe(id, ip.Unmap().String(), true)
}

func (m *Manager) subscribe(id, remoteIP string, authorize bool) (*Subscription, error) {
	m.operations.Lock()
	defer m.operations.Unlock()
	m.mu.Lock()
	defer m.mu.Unlock()
	if authorize {
		ip, _ := netip.ParseAddr(remoteIP)
		if _, allowed := m.playbackClients.Match(ip); !allowed {
			return nil, ErrClientDenied
		}
	}
	e := m.entries[id]
	if e == nil {
		return nil, ErrNotFound
	}
	if m.closed {
		return nil, ErrClosed
	}
	if m.ctx.Err() != nil {
		return nil, ErrClosed
	}
	if e.policyBlocked {
		return nil, ErrSourceDenied
	}
	if !e.channel.Enabled || e.blocked || m.options.Disabled {
		return nil, ErrDisabled
	}
	if w := e.worker; w != nil && w.stopping {
		m.mu.Unlock()
		<-w.done
		m.mu.Lock()
	}
	if m.totalClients >= m.options.MaxClients || len(e.clients) >= e.channel.MaxClients {
		return nil, ErrLimit
	}
	if _, err := m.validate(e.channel); err != nil {
		return nil, err
	}
	m.nextClient++
	s := &Subscription{manager: m, entry: e, id: m.nextClient, queue: make(chan []byte, 64), done: make(chan struct{}), remoteIP: remoteIP, connectedAt: time.Now().UTC()}
	e.clients[s.id] = s
	m.options.Logger.Info("playback client connected", "component", "playback", "channel_id", id, "event_type", "client_connect", "client_id", s.id)
	m.totalClients++
	m.startLocked(e, false)
	return s, nil
}
func (s *Subscription) Next(ctx context.Context) ([]byte, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-s.done:
		return nil, ErrClosed
	default:
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-s.done:
		return nil, ErrClosed
	case data := <-s.queue:
		m := s.manager
		m.mu.Lock()
		defer m.mu.Unlock()
		if s.closed {
			return nil, ErrClosed
		}
		s.queued -= len(data)
		s.entry.queued -= len(data)
		m.totalQueued -= len(data)
		return data, nil
	}
}
func (s *Subscription) Done() <-chan struct{} { return s.done }
func (s *Subscription) RecordOutput(n int) {
	if n > 0 {
		s.manager.mu.Lock()
		s.entry.counters.OutputBytes += uint64(n)
		s.delivered += uint64(n)
		s.manager.mu.Unlock()
	}
}
func (m *Manager) terminateLocked(s *Subscription) {
	if s.closed {
		return
	}
	s.closed = true
	m.options.Logger.Info("playback client disconnected", "component", "playback", "channel_id", s.entry.channel.ID, "event_type", "client_disconnect", "client_id", s.id)
	close(s.done)
	m.totalQueued -= s.queued
	s.entry.queued -= s.queued
	s.queued = 0
	for {
		select {
		case <-s.queue:
		default:
			if activeClients(s.entry) == 0 {
				s.entry.idleSince = time.Now()
			}
			return
		}
	}
}
func (s *Subscription) Close() {
	m := s.manager
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := s.entry.clients[s.id]; !exists {
		return
	}
	m.terminateLocked(s)
	delete(s.entry.clients, s.id)
	m.totalClients--
	if activeClients(s.entry) == 0 {
		s.entry.idleSince = time.Now()
	}
}
func (m *Manager) snapshotLocked(e *entry) Snapshot {
	now := time.Now()
	elapsed := now.Sub(e.sampleTime).Seconds()
	if elapsed >= 1 {
		e.inRate = float64(e.counters.InputBytes-e.sampleIn) * 8 / elapsed
		e.outRate = float64(e.counters.OutputBytes-e.sampleOut) * 8 / elapsed
		e.sampleIn = e.counters.InputBytes
		e.sampleOut = e.counters.OutputBytes
		e.sampleTime = now
	}
	uptime := int64(0)
	if e.worker != nil {
		uptime = int64(now.Sub(e.started).Seconds())
	}
	var last *time.Time
	if !e.lastPacket.IsZero() {
		v := e.lastPacket.UTC()
		last = &v
	}
	errorsCopy := append([]RecentError{}, e.errors...)
	var started *time.Time
	if e.worker != nil {
		value := e.started.UTC()
		started = &value
	}
	return Snapshot{Health: m.healthLocked(e), LastDatagramAt: timestamp(e.lastDatagram), LastRecoveryAt: timestamp(e.lastRecovery), SocketBound: e.socketBound, MembershipJoined: e.membershipJoined, StartedAt: started, Channel: e.channel, State: e.state, Clients: activeClients(e), UptimeSeconds: uptime, LastPacketAt: last, InputBitrate: e.inRate, OutputBitrate: e.outRate, Counters: e.counters, Errors: errorsCopy}
}
func (m *Manager) List() []Snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	items := make([]Snapshot, 0, len(m.entries))
	for _, e := range m.entries {
		items = append(items, m.snapshotLocked(e))
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Channel.ID < items[j].Channel.ID })
	return items
}
func (m *Manager) Get(id string) (Snapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e := m.entries[id]
	if e == nil {
		return Snapshot{}, ErrNotFound
	}
	return m.snapshotLocked(e), nil
}
func (m *Manager) Clients() []ClientSnapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	items := []ClientSnapshot{}
	for _, e := range m.entries {
		for _, s := range e.clients {
			elapsed := time.Since(s.connectedAt).Seconds()
			rate := float64(0)
			if elapsed > 0 {
				rate = float64(s.delivered) * 8 / elapsed
			}
			items = append(items, ClientSnapshot{ID: s.id, ChannelID: e.channel.ID, RemoteIP: s.remoteIP, ConnectedAt: s.connectedAt, QueuedBytes: s.queued, Closing: s.closed, DurationSeconds: int64(elapsed), BytesDelivered: s.delivered, OutputBitrate: rate})
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return items
}
func (m *Manager) FindMulticast(address string, port int) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := ""
	for _, e := range m.entries {
		c := e.channel
		if c.Enabled && c.SourceType == "multicast" && c.SourceAddress == address && c.SourcePort == port {
			if id != "" {
				return "", ErrConflict
			}
			id = c.ID
		}
	}
	if id == "" {
		return "", ErrNotFound
	}
	return id, nil
}
