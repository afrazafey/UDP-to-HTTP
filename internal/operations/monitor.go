package operations

import (
	"context"
	"runtime"
	"sync"
	"time"

	"github.com/udphttp-proxy/udphttp-proxy/internal/stream"
)

type Summary struct {
	Total            int     `json:"total"`
	Running          int     `json:"running"`
	Stopped          int     `json:"stopped"`
	Unhealthy        int     `json:"unhealthy"`
	Clients          int     `json:"clients"`
	InputBitrate     float64 `json:"input_bitrate"`
	OutputBitrate    float64 `json:"output_bitrate"`
	ContinuityErrors uint64  `json:"continuity_errors"`
	Discontinuities  uint64  `json:"discontinuities"`
	RecentErrors     int     `json:"recent_errors"`
	Inactive         int     `json:"inactive"`
	ActiveReceivers  int     `json:"active_receivers"`
}
type Sample struct {
	Time          time.Time `json:"time"`
	InputBitrate  float64   `json:"input_bitrate"`
	OutputBitrate float64   `json:"output_bitrate"`
	HeapBytes     uint64    `json:"heap_bytes"`
	RuntimeBytes  uint64    `json:"runtime_bytes"`
	RSSBytes      *uint64   `json:"rss_bytes"`
	CPUPercent    *float64  `json:"cpu_percent"`
	Goroutines    int       `json:"goroutines"`
	Receivers     int       `json:"receivers"`
	Clients       int       `json:"clients"`
}

func Aggregate(items []stream.Snapshot, receivers int) Summary {
	s := Summary{Total: len(items), ActiveReceivers: receivers}
	for _, item := range items {
		switch item.State {
		case "running":
			s.Running++
		case "stopped", "disabled":
			s.Stopped++
		case "retrying", "error":
			s.Unhealthy++
		case "waiting":
			s.Inactive++
		}
		s.Clients += item.Clients
		s.InputBitrate += item.InputBitrate
		s.OutputBitrate += item.OutputBitrate
		s.ContinuityErrors += item.Counters.ContinuityErrors
		s.Discontinuities += item.Counters.Discontinuities
		s.RecentErrors += len(item.Errors)
	}
	return s
}

type Monitor struct {
	mu       sync.Mutex
	engine   *stream.Manager
	history  []Sample
	streams  map[string][]Sample
	summary  *Summary
	lastCPU  *float64
	lastTime time.Time
	cancel   context.CancelFunc
	done     chan struct{}
}

func NewMonitor(ctx context.Context, engine *stream.Manager) *Monitor {
	ctx, cancel := context.WithCancel(ctx)
	m := &Monitor{engine: engine, history: []Sample{}, streams: map[string][]Sample{}, cancel: cancel, done: make(chan struct{})}
	m.collect()
	go func() {
		defer close(m.done)
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.collect()
			}
		}
	}()
	return m
}
func bounded(samples []Sample, s Sample, max int) []Sample {
	samples = append(samples, s)
	if len(samples) > max {
		samples = append([]Sample{}, samples[len(samples)-max:]...)
	}
	return samples
}
func (m *Monitor) collect() {
	now := time.Now().UTC()
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	cpu, rss := processUsage()
	sample := Sample{Time: now, HeapBytes: mem.Alloc, RuntimeBytes: mem.Sys, RSSBytes: rss, Goroutines: runtime.NumGoroutine()}
	var snapshots []stream.Snapshot
	if m.engine != nil {
		snapshots = m.engine.List()
		sample.Receivers = m.engine.ReceiverCount()
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if cpu != nil && m.lastCPU != nil && now.After(m.lastTime) {
		value := max(0, (*cpu-*m.lastCPU)/now.Sub(m.lastTime).Seconds()*100)
		sample.CPUPercent = &value
	}
	m.lastCPU = cpu
	m.lastTime = now
	if m.engine != nil {
		sum := Aggregate(snapshots, sample.Receivers)
		m.summary = &sum
		sample.InputBitrate = sum.InputBitrate
		sample.OutputBitrate = sum.OutputBitrate
		sample.Clients = sum.Clients
	}
	m.history = bounded(m.history, sample, 240)
	present := map[string]bool{}
	for _, item := range snapshots {
		present[item.Channel.ID] = true
		s := sample
		s.InputBitrate = item.InputBitrate
		s.OutputBitrate = item.OutputBitrate
		s.Clients = item.Clients
		m.streams[item.Channel.ID] = bounded(m.streams[item.Channel.ID], s, 120)
	}
	for id := range m.streams {
		if !present[id] {
			delete(m.streams, id)
		}
	}
}
func (m *Monitor) View() (Sample, []Sample, *Summary) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var sum *Summary
	if m.summary != nil {
		v := *m.summary
		sum = &v
	}
	return m.history[len(m.history)-1], append([]Sample{}, m.history...), sum
}
func (m *Monitor) StreamHistory(id string) []Sample {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]Sample{}, m.streams[id]...)
}
func (m *Monitor) Close() { m.cancel(); <-m.done }
