package events

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
)

type memoryEvents struct {
	mu    sync.Mutex
	items []Entry
}

func (m *memoryEvents) AppendEvent(_ context.Context, e Entry, _, _ int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.items = append(m.items, e)
	return nil
}
func (m *memoryEvents) QueryEvents(context.Context, Filter) ([]Entry, int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]Entry{}, m.items...), len(m.items), nil
}
func TestRedactionAndStructuredCapture(t *testing.T) {
	repo := &memoryEvents{}
	sink := NewSink(repo, 100, 7)
	defer sink.Close()
	var output bytes.Buffer
	logger := slog.New(NewHandler(slog.NewJSONHandler(&output, nil), sink)).With("component", "test")
	logger.Warn("source https://user:private@example.test/live?token=private", "channel_id", "news", "password", "private", "context", "token=private")
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		items, _, _ := repo.QueryEvents(context.Background(), Filter{})
		if len(items) > 0 {
			if items[0].StreamID != "news" || items[0].Component != "test" || items[0].Level != "warn" {
				t.Fatal(items)
			}
			if strings.Contains(output.String(), "private") {
				t.Fatal("secret leaked", output.String())
			}
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("event was not captured")
}
