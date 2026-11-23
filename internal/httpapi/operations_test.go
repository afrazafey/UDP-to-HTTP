package httpapi

import (
	"context"
	"io"
	"log/slog"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/udphttp-proxy/udphttp-proxy/internal/events"
	"github.com/udphttp-proxy/udphttp-proxy/internal/operations"
	"github.com/udphttp-proxy/udphttp-proxy/internal/storage"
)

func operationsService(t *testing.T, configure ...func(*Service)) *Service {
	t.Helper()
	s, engine, _ := streamingService(t)
	s.options.Config.Streaming.Enabled = false
	for _, change := range configure {
		change(s)
	}
	store, err := storage.Open(context.Background(), filepath.Join(t.TempDir(), "phase3.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	manager, err := operations.NewSettings(context.Background(), s.options.Config, store)
	if err != nil {
		t.Fatal(err)
	}
	monitor := operations.NewMonitor(context.Background(), engine)
	t.Cleanup(monitor.Close)
	if err = store.AppendEvent(context.Background(), events.Entry{Time: time.Now().UTC(), Level: "warn", Component: "stream", StreamID: "fixture", Message: "receiver inactive", Context: map[string]any{"safe": true}}, 100, 7); err != nil {
		t.Fatal(err)
	}
	return New(Options{Config: s.options.Config, Store: store, Logger: slog.New(slog.NewJSONHandler(io.Discard, nil)), Streams: engine, Settings: manager, Monitor: monitor, Events: store, ConfigPath: "fixture.yaml", Listeners: []ListenerStatus{{Kind: "management", Address: "127.0.0.1:8080", State: "listening"}}})
}
func TestOperationsEndpointsAndAuthorization(t *testing.T) {
	s := operationsService(t)
	for _, path := range []string{"/api/v1/dashboard", "/api/v1/metrics", "/api/v1/system", "/api/v1/settings", "/api/v1/logs?level=warn", "/api/v1/logs?search=receiver", "/api/v1/logs/export"} {
		w := managementRequest(t, s, "GET", path, nil, false)
		if w.Code != 200 {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body)
		}
	}
	for _, path := range []string{"/api/v1/logs?level=invalid", "/api/v1/logs?limit=1000", "/api/v1/logs?offset=-1", "/api/v1/logs?since=invalid"} {
		if w := managementRequest(t, s, "GET", path, nil, false); w.Code != 400 {
			t.Fatal(path, w.Code)
		}
	}
	r := req("GET", "/api/v1/system")
	r.RemoteAddr = "10.0.0.5:1000"
	r.Header.Set("X-Forwarded-For", "127.0.0.1")
	remote := httptest.NewRecorder()
	s.Handler().ServeHTTP(remote, r)
	if remote.Code != 403 {
		t.Fatal("remote management was authorized")
	}
	w := httptest.NewRecorder()
	w = managementRequest(t, s, "POST", "/api/v1/settings/validate", map[string]any{}, false)
	if w.Code != 403 {
		t.Fatal("mutation without token accepted")
	}
}
func TestSettingsPreviewSaveConflictsAndFailures(t *testing.T) {
	s := operationsService(t)
	value := operations.FromConfig(s.options.Config)
	value.Dashboard.Name = "Updated operations"
	body := map[string]any{"revision": 0, "settings": value, "confirm": false}
	if w := managementRequest(t, s, "POST", "/api/v1/settings/validate", body, true); w.Code != 200 {
		t.Fatalf("preview %d %s", w.Code, w.Body)
	}
	if w := managementRequest(t, s, "PUT", "/api/v1/settings", body, true); w.Code != 400 {
		t.Fatal("save without confirmation accepted")
	}
	body["confirm"] = true
	if w := managementRequest(t, s, "PUT", "/api/v1/settings", body, true); w.Code != 200 {
		t.Fatalf("save %d %s", w.Code, w.Body)
	}
	if w := managementRequest(t, s, "PUT", "/api/v1/settings", body, true); w.Code != 409 {
		t.Fatal("stale revision accepted", w.Code)
	}
	body["revision"] = 1
	value.Network.HTTPPort = 8080
	body["settings"] = value
	if w := managementRequest(t, s, "PUT", "/api/v1/settings", body, true); w.Code != 400 {
		t.Fatal("bad listener settings accepted", w.Code)
	}
	view, err := s.options.Settings.View(context.Background())
	if err != nil || view["revision"].(int64) != 1 {
		t.Fatal("failed save changed settings", view, err)
	}
}
func TestConcurrentOperationsReads(t *testing.T) {
	s := operationsService(t)
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for _, path := range []string{"/api/v1/dashboard", "/api/v1/metrics", "/api/v1/settings", "/api/v1/logs"} {
				if w := managementRequest(t, s, "GET", path, nil, false); w.Code != 200 {
					t.Errorf("%s: %d", path, w.Code)
				}
			}
		}()
	}
	wg.Wait()
}
