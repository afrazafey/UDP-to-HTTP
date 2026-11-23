package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/udphttp-proxy/udphttp-proxy/internal/buildinfo"
	"github.com/udphttp-proxy/udphttp-proxy/internal/config"
	"github.com/udphttp-proxy/udphttp-proxy/internal/events"
	"github.com/udphttp-proxy/udphttp-proxy/internal/httpapi"
	"github.com/udphttp-proxy/udphttp-proxy/internal/operations"
	"github.com/udphttp-proxy/udphttp-proxy/internal/storage"
	"github.com/udphttp-proxy/udphttp-proxy/internal/stream"
	"github.com/udphttp-proxy/udphttp-proxy/internal/web"
)

func bindPlayback(ctx context.Context, c config.Config) ([]net.Listener, error) {
	addresses, err := operations.PlaybackAddresses(c)
	if err != nil {
		return nil, err
	}
	listeners := []net.Listener{}
	for _, address := range addresses {
		l, e := (&net.ListenConfig{}).Listen(ctx, "tcp4", address)
		if e != nil {
			for _, open := range listeners {
				_ = open.Close()
			}
			return nil, fmt.Errorf("playback listen %s: %w", address, e)
		}
		listeners = append(listeners, l)
	}
	return listeners, nil
}
func configurationPath(args []string, getenv func(string) string) string {
	path := ""
	if getenv != nil {
		path = getenv("UDPHTTP_CONFIG")
	}
	for i, arg := range args {
		if strings.HasPrefix(arg, "--config=") || strings.HasPrefix(arg, "-config=") {
			path = strings.SplitN(arg, "=", 2)[1]
		}
		if (arg == "--config" || arg == "-config") && i+1 < len(args) {
			path = args[i+1]
		}
	}
	if path != "" {
		if absolute, e := filepath.Abs(path); e == nil {
			path = absolute
		}
	}
	return path
}
func serve(ctx context.Context, args []string, errOut io.Writer, getenv func(string) string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	base, err := loadFlags("serve", args, errOut, getenv)
	if err != nil {
		return err
	}
	store, err := storage.Open(ctx, base.Storage.Path)
	if err != nil {
		return err
	}
	defer store.Close()
	settings, err := operations.NewSettings(ctx, base, store)
	if err != nil {
		return err
	}
	ignore := getenv != nil && getenv("UDPHTTP_IGNORE_SAVED_SETTINGS") == "1"
	cfg, err := settings.Startup(ctx, ignore)
	if err != nil {
		return err
	}
	var level slog.LevelVar
	var selected slog.Level
	_ = selected.UnmarshalText([]byte(cfg.Logging.Level))
	level.Set(selected)
	sink := events.NewSink(store, cfg.Logging.RetentionCount, cfg.Logging.RetentionDays)
	defer sink.Close()
	var output slog.Handler = slog.NewJSONHandler(errOut, &slog.HandlerOptions{Level: &level})
	if cfg.LogFormat == "text" {
		output = slog.NewTextHandler(errOut, &slog.HandlerOptions{Level: &level})
	}
	logger := slog.New(events.NewHandler(output, sink)).With("service", "udphttp-proxy")
	settings.Live = func(_ config.Dashboard, l config.Logging) {
		var value slog.Level
		_ = value.UnmarshalText([]byte(l.Level))
		level.Set(value)
		sink.SetRetention(l.RetentionCount, l.RetentionDays)
		pruneCtx, pruneCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer pruneCancel()
		if e := store.PruneEvents(pruneCtx, l.RetentionCount, l.RetentionDays); e != nil {
			logger.Error("operational retention update failed", "component", "logging", "error", e)
		}
	}
	if err = store.SeedChannels(ctx, base.Channels); err != nil {
		return err
	}
	address := net.JoinHostPort(cfg.Management.Host, strconv.Itoa(cfg.Management.Port))
	management, err := (&net.ListenConfig{}).Listen(ctx, "tcp4", address)
	if err != nil {
		return fmt.Errorf("management listen: %w", err)
	}
	listeners := []net.Listener{management}
	defer func() {
		for _, l := range listeners {
			_ = l.Close()
		}
	}()
	statuses := []httpapi.ListenerStatus{{Kind: "management", Address: management.Addr().String(), State: "listening"}}
	playback, bindErr := bindPlayback(ctx, cfg)
	accept := !ignore
	if bindErr != nil {
		accept = false
		logger.Error("requested playback configuration failed; restoring accepted settings", "component", "configuration", "error", bindErr)
		statuses = append(statuses, httpapi.ListenerStatus{Kind: "playback", Address: "requested configuration", State: "failed", Error: events.Redact(bindErr.Error())})
		if !ignore {
			if e := settings.Reject(ctx, bindErr.Error()); e != nil {
				return e
			}
		}
		fallback, e := settings.Accepted(ctx)
		if e != nil {
			return e
		}
		if ignore {
			fallback = base
		}
		cfg = fallback
		playback, bindErr = bindPlayback(ctx, cfg)
		if bindErr != nil {
			logger.Error("playback unavailable; local management remains available", "component", "http", "error", bindErr)
			cfg.Streaming.Enabled = false
			accept = false
			playback = nil
		}
	}
	for _, l := range playback {
		listeners = append(listeners, l)
		statuses = append(statuses, httpapi.ListenerStatus{Kind: "playback", Address: l.Addr().String(), State: "listening"})
		logger.Info("playback listening", "component", "http", "address", l.Addr().String())
	}
	policy := stream.RecoveryPolicy{Enabled: cfg.Recovery.Enabled, MinDelay: cfg.Recovery.MinDelay.Value(), MaxDelay: cfg.Recovery.MaxDelay.Value(), RejoinAfter: cfg.Recovery.RejoinAfter.Value(), Jitter: cfg.Recovery.Jitter, MaxAttempts: cfg.Recovery.MaxAttempts}
	engine, err := stream.NewManager(ctx, stream.Options{Recovery: &policy, Policy: cfg.StreamPolicy(), Repository: store, Logger: logger.With("component", "stream"), MaxClients: cfg.Streaming.MaxClients, MaxStreams: cfg.Streaming.MaxStreams, ClientBufferBytes: cfg.Streaming.ClientBufferBytes, UDPReceiveBuffer: cfg.Streaming.UDPReceiveBuffer, StreamBuffer: cfg.Streaming.StreamBuffer, MaxBufferedBytes: cfg.Streaming.MaxBufferedBytes, InactivityTimeout: cfg.Streaming.InactivityTimeout.Value(), Disabled: !cfg.Streaming.Enabled})
	if err != nil {
		return err
	}
	defer engine.Close()
	if err = settings.Activate(ctx, cfg, accept); err != nil {
		return err
	}
	owned := []string{}
	for _, l := range listeners {
		owned = append(owned, l.Addr().String())
	}
	settings.SetListeners(owned)
	monitor := operations.NewMonitor(ctx, engine)
	defer monitor.Close()
	snapshot, _ := json.Marshal(cfg)
	if err = store.Put(ctx, "last_startup_configuration", string(snapshot)); err != nil {
		return err
	}
	logger.Info("management listening", "component", "http", "address", management.Addr().String(), "phase", 4, "version", buildinfo.Version, "saved_settings_ignored", ignore)
	service := httpapi.New(httpapi.Options{Config: cfg, Store: store, Logger: logger, Assets: web.Assets(), Streams: engine, Settings: settings, Monitor: monitor, Events: store, DroppedLogs: func() uint64 { return sink.Dropped.Load() }, ConfigPath: configurationPath(args, getenv), Listeners: statuses})
	results := make(chan error, len(listeners))
	for i, l := range listeners {
		go func(index int, l net.Listener) {
			if index == 0 {
				results <- service.Serve(ctx, l)
			} else {
				results <- service.ServePlayback(ctx, l)
			}
		}(i, l)
	}
	first := <-results
	cancel()
	for i := 1; i < len(listeners); i++ {
		if e := <-results; first == nil && e != nil {
			first = e
		}
	}
	if first != nil {
		logger.Error("HTTP listener failure", "component", "http", "error", first)
	}
	return first
}
