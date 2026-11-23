package operations

import (
	"context"
	"testing"

	"github.com/udphttp-proxy/udphttp-proxy/internal/stream"
)

func TestAggregate(t *testing.T) {
	items := []stream.Snapshot{{State: "running", Clients: 2, InputBitrate: 1000, OutputBitrate: 2000, Counters: stream.Counters{ContinuityErrors: 3, Discontinuities: 2}}, {State: "stopped"}, {State: "disabled"}, {State: "retrying"}, {State: "waiting"}, {State: "error"}}
	got := Aggregate(items, 3)
	if got.Total != 6 || got.Running != 1 || got.Stopped != 2 || got.Unhealthy != 2 || got.Inactive != 1 || got.Clients != 2 || got.InputBitrate != 1000 || got.OutputBitrate != 2000 || got.ContinuityErrors != 3 || got.Discontinuities != 2 || got.ActiveReceivers != 3 {
		t.Fatalf("unexpected aggregate: %+v", got)
	}
}
func TestBoundedMonitor(t *testing.T) {
	m := NewMonitor(context.Background(), nil)
	m.Close()
	for i := 0; i < 250; i++ {
		m.collect()
	}
	sample, history, summary := m.View()
	if len(history) != 240 || sample.Goroutines < 1 || sample.RuntimeBytes == 0 || summary != nil {
		t.Fatalf("invalid metrics bounds: %d %+v", len(history), sample)
	}
	history[0].Goroutines = -1
	_, fresh, _ := m.View()
	if fresh[0].Goroutines == -1 {
		t.Fatal("history response aliases mutable storage")
	}
}
