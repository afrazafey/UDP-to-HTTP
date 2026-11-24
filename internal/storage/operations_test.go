package storage

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/udphttp-proxy/udphttp-proxy/internal/config"
	"github.com/udphttp-proxy/udphttp-proxy/internal/events"
	"github.com/udphttp-proxy/udphttp-proxy/internal/operations"
	"github.com/udphttp-proxy/udphttp-proxy/internal/stream"
)

func TestConcurrentSettingsWritesKeepOneRevision(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "settings.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	current := operations.FromConfig(config.Default())
	if err := store.InitServerSettings(ctx, current); err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	for _, name := range []string{"Fixture One", "Fixture Two"} {
		go func() {
			next := current
			next.Dashboard.Name = name
			<-start
			_, err := store.WriteServerSettings(ctx, 0, next)
			results <- err
		}()
	}
	close(start)
	writes := 0
	for range 2 {
		switch err := <-results; {
		case err == nil:
			writes++
		case errors.Is(err, operations.ErrRevision):
		default:
			t.Fatal(err)
		}
	}
	record, err := store.ReadServerSettings(ctx)
	if err != nil || writes != 1 || record.Revision != 1 {
		t.Fatalf("writes %d, record revision %d, error %v", writes, record.Revision, err)
	}
}

func TestSettingsRevisionsPersistenceAndRollback(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "state.db")
	s, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	base := config.Default()
	base.Streaming.Enabled = false
	m, err := operations.NewSettings(ctx, base, s)
	if err != nil {
		t.Fatal(err)
	}
	next := operations.FromConfig(base)
	next.Dashboard.Name = "Persisted name"
	r, err := m.Save(ctx, 0, next, false)
	if err != nil || r.Revision != 1 {
		t.Fatal(r, err)
	}
	if _, err = m.Save(ctx, 0, next, false); !errors.Is(err, operations.ErrRevision) {
		t.Fatal("stale revision accepted", err)
	}
	if _, err = m.Rollback(ctx, 0); !errors.Is(err, operations.ErrRevision) {
		t.Fatal("stale rollback accepted", err)
	}
	s.Close()
	s, err = Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	m, err = operations.NewSettings(ctx, base, s)
	if err != nil {
		t.Fatal(err)
	}
	candidate, err := m.Startup(ctx, false)
	if err != nil || candidate.Dashboard.Name != "Persisted name" {
		t.Fatal(candidate, err)
	}
	if err = m.Activate(ctx, candidate, true); err != nil {
		t.Fatal(err)
	}
	next.Dashboard.Name = "Rejected"
	if _, err = m.Save(ctx, 1, next, false); err != nil {
		t.Fatal(err)
	}
	if err = m.Reject(ctx, "port unavailable"); err != nil {
		t.Fatal(err)
	}
	r, err = s.ReadServerSettings(ctx)
	if err != nil || r.Desired.Dashboard.Name != "Persisted name" || r.LastError != "port unavailable" {
		t.Fatal(r, err)
	}
}
func TestSettingsValidationPreservesSavedValue(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, filepath.Join(t.TempDir(), "settings.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	base := config.Default()
	m, err := operations.NewSettings(ctx, base, s)
	if err != nil {
		t.Fatal(err)
	}
	invalid := operations.FromConfig(base)
	invalid.Network.HTTPPort = base.Management.Port
	if _, err = m.Save(ctx, 0, invalid, false); err == nil {
		t.Fatal("listener collision accepted")
	}
	r, err := s.ReadServerSettings(ctx)
	if err != nil || r.Revision != 0 || r.Desired.Network.HTTPPort != 7781 {
		t.Fatal("invalid save changed persisted config", r, err)
	}
	invalid = operations.FromConfig(base)
	invalid.Dashboard.RefreshSeconds = 0
	if _, err = m.Save(ctx, 0, invalid, false); err == nil {
		t.Fatal("invalid refresh accepted")
	}
}
func TestEventRetentionFilteringAndPagination(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, filepath.Join(t.TempDir(), "events.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	now := time.Now().UTC()
	for i := 0; i < 130; i++ {
		level := "info"
		if i%2 == 0 {
			level = "warn"
		}
		e := events.Entry{Time: now.Add(time.Duration(i) * time.Millisecond), Level: level, Component: "stream", StreamID: "news", Message: "receiver event", Context: map[string]any{"index": i}}
		if err = s.AppendEvent(ctx, e, 100, 7); err != nil {
			t.Fatal(err)
		}
	}
	items, total, err := s.QueryEvents(ctx, events.Filter{Level: "warn", StreamID: "news", Search: "receiver", Since: now, Limit: 10, Offset: 10})
	if err != nil || total != 50 || len(items) != 10 {
		t.Fatal(total, len(items), err)
	}
	_, total, err = s.QueryEvents(ctx, events.Filter{Limit: 100})
	if err != nil || total != 100 {
		t.Fatal("retention failed", total, err)
	}
}
func TestStartupRejectsStagedLimitBelowNewChannelCount(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, filepath.Join(t.TempDir(), "limit.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	base := config.Default()
	base.Streaming.Enabled = false
	base.Streaming.AllowedMulticastGroups = []string{"239.1.1.1"}
	m, err := operations.NewSettings(ctx, base, s)
	if err != nil {
		t.Fatal(err)
	}
	candidate := operations.FromConfig(base)
	candidate.Streaming.MaxStreams = 1
	if _, err = m.Save(ctx, 0, candidate, false); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"one", "two"} {
		if err = s.SaveChannel(ctx, stream.Channel{ID: id, Name: id, SourceType: "multicast", SourceAddress: "239.1.1.1", SourcePort: 1234, Interface: "eth0"}, true); err != nil {
			t.Fatal(err)
		}
	}
	restored, err := m.Startup(ctx, false)
	if err != nil || restored.Streaming.MaxStreams != 128 {
		t.Fatal("unsafe limit did not roll back", restored.Streaming.MaxStreams, err)
	}
	r, _ := s.ReadServerSettings(ctx)
	if r.LastError == "" {
		t.Fatal("rollback reason missing")
	}
}

func TestNetworkPoliciesReloadWithoutSchemaOrChannelChanges(t *testing.T) {
	ctx := t.Context()
	path := filepath.Join(t.TempDir(), "network.db")
	s, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	base := config.Default()
	base.Streaming.Enabled = false
	m, err := operations.NewSettings(ctx, base, s)
	if err != nil {
		t.Fatal(err)
	}
	channel := stream.Channel{ID: "existing", Name: "Existing channel", Group: "Music", SourceType: "multicast", SourceAddress: "239.1.1.61", SourcePort: 1234, Interface: "eth0", Enabled: true}
	if err := s.SaveChannel(ctx, channel, true); err != nil {
		t.Fatal(err)
	}
	draft := operations.FromConfig(base)
	draft.Streaming.AllowedClientNetworks = []string{"10.14.0.5, 10.14.0.5/32"}
	draft.Streaming.AllowedMulticastGroups = []string{"239.1.1.61/24"}
	if _, err := m.Save(ctx, 0, draft, false); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	m, err = operations.NewSettings(ctx, base, s)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := m.Startup(ctx, false)
	if err != nil || len(cfg.Streaming.AllowedClientNetworks) != 1 || cfg.Streaming.AllowedClientNetworks[0] != "10.14.0.5" ||
		len(cfg.Streaming.AllowedMulticastGroups) != 1 || cfg.Streaming.AllowedMulticastGroups[0] != "239.1.1.0/24" {
		t.Fatalf("restored configuration: %+v %v", cfg.Streaming, err)
	}
	if channels, err := s.ListChannels(ctx); err != nil || len(channels) != 1 || channels[0] != channel {
		t.Fatalf("channel metadata changed: %+v %v", channels, err)
	}
	if version, err := s.SchemaVersion(ctx); err != nil || version != 4 {
		t.Fatalf("unexpected schema %d: %v", version, err)
	}
}
