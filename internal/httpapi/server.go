package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/udphttp-proxy/udphttp-proxy/internal/buildinfo"
	"github.com/udphttp-proxy/udphttp-proxy/internal/config"
	"github.com/udphttp-proxy/udphttp-proxy/internal/events"
	"github.com/udphttp-proxy/udphttp-proxy/internal/network"
	"github.com/udphttp-proxy/udphttp-proxy/internal/operations"
	"github.com/udphttp-proxy/udphttp-proxy/internal/stream"
)

type HealthStore interface{ Ping(context.Context) error }
type Options struct {
	Config      config.Config
	Store       HealthStore
	Logger      *slog.Logger
	Assets      fs.FS
	Discover    func() ([]network.Interface, error)
	Streams     *stream.Manager
	Settings    *operations.SettingsManager
	Monitor     *operations.Monitor
	Events      events.Repository
	DroppedLogs func() uint64
	ConfigPath  string
	Listeners   []ListenerStatus
}
type Service struct {
	options     Options
	started     time.Time
	handler     http.Handler
	csrf        string
	writeMu     sync.Mutex
	writeWindow time.Time
	writes      int
}
type ListenerStatus struct {
	Kind    string `json:"kind"`
	Address string `json:"address"`
	State   string `json:"state"`
	Error   string `json:"error,omitempty"`
}

func New(o Options) *Service {
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	if o.Discover == nil {
		o.Discover = network.Discover
	}
	s := &Service{options: o, started: buildinfo.StartedAt, csrf: rand.Text()}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/playlist.m3u", s.exportPlaylist)
	mux.HandleFunc("/api/v1/playlist/config", s.playlistConfiguration)
	mux.HandleFunc("/api/v1/import/", s.channelImport)
	mux.HandleFunc("/api/v1/streams/bulk", s.bulkChannels)
	mux.HandleFunc("/api/v1/diagnostics", s.diagnostics)
	mux.HandleFunc("/health", s.health)
	mux.HandleFunc("/api/v1/status", s.status)
	mux.HandleFunc("/api/v1/interfaces", s.interfaces)
	mux.HandleFunc("/api/v1/csrf", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, map[string]string{"token": s.csrf}) })
	mux.HandleFunc("/api/v1/streams", s.streamManagement)
	mux.HandleFunc("/api/v1/streams/", s.streamManagement)
	mux.HandleFunc("/api/v1/clients", s.connectedClients)
	mux.HandleFunc("/api/v1/clients/", s.clientAction)
	mux.HandleFunc("/api/v1/dashboard", s.operationsDashboard)
	mux.HandleFunc("/api/v1/metrics", s.metrics)
	mux.HandleFunc("/api/v1/system", s.systemInfo)
	mux.HandleFunc("/api/v1/logs", s.logs)
	mux.HandleFunc("/api/v1/logs/export", s.logs)
	mux.HandleFunc("/api/v1/settings", s.settings)
	mux.HandleFunc("/api/v1/settings/validate", s.settings)
	mux.HandleFunc("/api/v1/settings/rollback", s.settings)
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 404, map[string]string{"error": "API endpoint not found"})
	})
	unavailable := func(w http.ResponseWriter, r *http.Request) {
		if s.options.Streams != nil {
			writeJSON(w, 404, map[string]string{"error": "Use the separate playback listener for streaming URLs"})
			return
		}
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "Streaming is unavailable in Phase 1"})
	}
	mux.HandleFunc("/udp/", unavailable)
	mux.HandleFunc("/live/", unavailable)
	mux.HandleFunc("/", s.dashboard)
	s.handler = s.protect(mux)
	return s
}
func (s *Service) Handler() http.Handler { return s.handler }
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// Management always uses a loopback listener. Proxy trust is explicitly opt-in;
// both direct and proxied requests pass browser-origin checks before routing.
func (s *Service) protect(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'")
		if err := s.authorizeManagement(r); err != nil {
			writeJSON(w, 403, map[string]string{"error": err.Error()})
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			capable := s.options.Streams != nil && (r.URL.Path == "/api/v1/streams" || strings.HasPrefix(r.URL.Path, "/api/v1/streams/") || strings.HasPrefix(r.URL.Path, "/api/v1/clients/")) || s.options.Settings != nil && (r.URL.Path == "/api/v1/settings" || strings.HasPrefix(r.URL.Path, "/api/v1/settings/"))
			capable = capable || s.options.Streams != nil && (strings.HasPrefix(r.URL.Path, "/api/v1/import/") || r.URL.Path == "/api/v1/diagnostics")
			mutation := capable && (r.Method == "POST" || r.Method == "PUT" || r.Method == "DELETE")
			if mutation {
				token, err := singleHeader(r, "X-CSRF-Token")
				if err != nil || subtle.ConstantTimeCompare([]byte(token), []byte(s.csrf)) != 1 {
					writeJSON(w, 403, map[string]string{"error": "A valid local CSRF token is required"})
					return
				}
				if r.Header.Get("Content-Type") != "application/json" {
					writeJSON(w, 415, map[string]string{"error": "Content-Type must be application/json"})
					return
				}
				if r.ContentLength > 1<<20 {
					writeJSON(w, 413, map[string]string{"error": "Management request exceeds 1 MiB"})
					return
				}
				r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
				s.writeMu.Lock()
				if time.Since(s.writeWindow) >= time.Minute {
					s.writeWindow = time.Now()
					s.writes = 0
				}
				s.writes++
				limited := s.writes > 60
				s.writeMu.Unlock()
				if limited {
					w.Header().Set("Retry-After", "60")
					writeJSON(w, 429, map[string]string{"error": "Management write limit reached; retry after one minute"})
					return
				}
				next.ServeHTTP(w, r)
				return
			}
			w.Header().Set("Allow", "GET, HEAD")
			writeJSON(w, 405, map[string]string{"error": "Read-only Phase 1 API"})
			return
		}
		next.ServeHTTP(w, r)
	})
}
func (s *Service) databaseOK(ctx context.Context) bool {
	if s.options.Store == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return s.options.Store.Ping(ctx) == nil
}
func (s *Service) health(w http.ResponseWriter, r *http.Request) {
	ok := s.databaseOK(r.Context())
	code := http.StatusOK
	status := "ok"
	if !ok {
		code = 503
		status = "degraded"
	}
	phase := 1
	if s.options.Streams != nil {
		phase = 2
	}
	if s.options.Settings != nil {
		phase = 4
	}
	writeJSON(w, code, map[string]any{"status": status, "phase": phase, "storage_ready": ok})
}
func (s *Service) status(w http.ResponseWriter, r *http.Request) {
	ok := s.databaseOK(r.Context())
	status := "running"
	if !ok {
		status = "degraded"
	}
	phase := 1
	streaming := "unavailable"
	metrics := "unavailable"
	editor := "unavailable"
	if s.options.Streams != nil {
		phase = 2
		streaming = "available"
		metrics = "available"
		editor = "channels_only"
		if !s.options.Config.Streaming.Enabled {
			streaming = "disabled"
		}
	}
	logViewer := "unavailable"
	if s.options.Settings != nil {
		phase = 4
		editor = "settings_and_channels"
	}
	if s.options.Events != nil {
		logViewer = "available"
	}
	activeConfig := s.options.Config
	if s.options.Settings != nil {
		activeConfig = s.options.Settings.Active()
	}
	remote := "unavailable"
	if s.options.Config.Management.ReverseProxy.Enabled {
		remote = "trusted_reverse_proxy"
	}
	writeJSON(w, 200, map[string]any{
		"service": "udphttp-proxy", "version": buildinfo.Version, "commit": buildinfo.Commit, "build_date": buildinfo.Date, "phase": phase,
		"status": status, "started_at": s.started, "uptime_seconds": int64(time.Since(s.started).Seconds()), "platform": runtime.GOOS + "/" + runtime.GOARCH, "go_version": runtime.Version(),
		"components":    map[string]string{"management": "available", "storage": map[bool]string{true: "ready", false: "unavailable"}[ok], "streaming": streaming, "remote_administration": remote, "metrics": metrics, "log_viewer": logViewer, "configuration_editor": editor},
		"configuration": activeConfig,
	})
}
func (s *Service) interfaces(w http.ResponseWriter, r *http.Request) {
	items, err := s.options.Discover()
	if err != nil {
		s.options.Logger.Error("interface discovery failed", "error", err)
		writeJSON(w, 503, map[string]string{"error": "Interface discovery is unavailable"})
		return
	}
	writeJSON(w, 200, map[string]any{"interfaces": items})
}
func (s *Service) dashboard(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		path = "index.html"
	}
	if !fs.ValidPath(path) {
		http.NotFound(w, r)
		return
	}
	if s.options.Assets != nil {
		if _, err := fs.Stat(s.options.Assets, path); err == nil {
			http.FileServer(http.FS(s.options.Assets)).ServeHTTP(w, r)
			return
		}
	}
	if path != "index.html" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusServiceUnavailable)
	_, _ = fmt.Fprint(w, `<!doctype html><html lang="en"><meta charset="utf-8"><title>UDPHTTP · Dashboard unavailable</title><h1>Dashboard assets are unavailable</h1><p>Build the frontend before building the Go binary: cd frontend &amp;&amp; npm ci &amp;&amp; npm run build</p><p>The status API is available at <a href="/api/v1/status">/api/v1/status</a>.</p></html>`)
}

// Serve owns its listener and waits for shutdown to complete before returning.
func (s *Service) Serve(ctx context.Context, listener net.Listener) error {
	c := s.options.Config.Management
	server := &http.Server{Handler: s.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: c.ReadTimeout.Value(), WriteTimeout: c.WriteTimeout.Value(), IdleTimeout: c.IdleTimeout.Value(), MaxHeaderBytes: 16 << 10, BaseContext: func(net.Listener) context.Context { return ctx }, ErrorLog: slog.NewLogLogger(s.options.Logger.Handler(), slog.LevelError)}
	defer server.Close()
	result := make(chan error, 1)
	go func() { result <- server.Serve(listener) }()
	select {
	case err := <-result:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		s.options.Logger.Info("shutdown requested")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), c.ShutdownTimeout.Value())
		defer cancel()
		err := server.Shutdown(shutdownCtx)
		if err != nil {
			_ = server.Close()
		}
		serveErr := <-result
		if err != nil {
			return fmt.Errorf("graceful shutdown: %w", err)
		}
		if !errors.Is(serveErr, http.ErrServerClosed) {
			return serveErr
		}
		return nil
	}
}
