package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"github.com/udphttp-proxy/udphttp-proxy/internal/stream"
	"golang.org/x/net/netutil"
)

const playbackConnectionHeadroom = 32

func streamError(w http.ResponseWriter, err error) {
	code := 400
	var tooLarge *http.MaxBytesError
	switch {
	case errors.As(err, &tooLarge):
		code = 413
	case errors.Is(err, stream.ErrNotFound):
		code = 404
	case errors.Is(err, stream.ErrConflict):
		code = 409
	case errors.Is(err, stream.ErrLimit):
		code = 429
	case errors.Is(err, stream.ErrDisabled), errors.Is(err, stream.ErrClosed):
		code = 503
	case errors.Is(err, stream.ErrStorage):
		writeJSON(w, 500, map[string]string{"error": "Channel persistence failed; check service logs and database permissions"})
		return
	}
	writeJSON(w, code, map[string]string{"error": err.Error()})
}
func decodeChannel(r *http.Request) (stream.Channel, error) {
	var c stream.Channel
	err := decodeStrict(r, &c)
	return c, err
}
func (s *Service) connectedClients(w http.ResponseWriter, r *http.Request) {
	if s.options.Streams == nil {
		writeJSON(w, 503, map[string]string{"error": "Streaming engine is unavailable"})
		return
	}
	writeJSON(w, 200, map[string]any{"clients": s.options.Streams.Clients()})
}
func (s *Service) streamManagement(w http.ResponseWriter, r *http.Request) {
	m := s.options.Streams
	if m == nil {
		writeJSON(w, 503, map[string]string{"error": "Streaming engine is unavailable"})
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/streams")
	if path == "" {
		switch r.Method {
		case "GET", "HEAD":
			writeJSON(w, 200, map[string]any{"streams": m.List()})
		case "POST":
			c, err := decodeChannel(r)
			if err != nil {
				streamError(w, err)
				return
			}
			if err = m.Put(r.Context(), c, true); err != nil {
				streamError(w, err)
				return
			}
			snapshot, _ := m.Get(c.ID)
			s.options.Logger.Info("channel created", "component", "streams", "channel_id", c.ID)
			w.Header().Set("Location", "/api/v1/streams/"+c.ID)
			writeJSON(w, 201, map[string]any{"stream": snapshot})
		default:
			w.Header().Set("Allow", "GET, HEAD, POST")
			writeJSON(w, 405, map[string]string{"error": "Method not allowed"})
		}
		return
	}
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(parts) > 2 || parts[0] == "" {
		writeJSON(w, 404, map[string]string{"error": "Unknown stream endpoint"})
		return
	}
	id := parts[0]
	if len(parts) == 2 {
		switch parts[1] {
		case "start", "stop", "restart":
			if r.Method != "POST" {
				w.Header().Set("Allow", "POST")
				writeJSON(w, 405, map[string]string{"error": "Use POST for stream actions"})
				return
			}
			if err := m.Action(id, parts[1]); err != nil {
				streamError(w, err)
				return
			}
			v, _ := m.Get(id)
			s.options.Logger.Info("channel "+parts[1]+" confirmed", "component", "streams", "channel_id", id)
			writeJSON(w, 202, map[string]any{"stream": v})
			return
		case "status", "errors", "metrics":
			if r.Method != "GET" && r.Method != "HEAD" {
				writeJSON(w, 405, map[string]string{"error": "Read-only stream endpoint"})
				return
			}
			v, err := m.Get(id)
			if err != nil {
				streamError(w, err)
				return
			}
			if parts[1] == "metrics" {
				if s.options.Monitor == nil {
					unavailable(w)
					return
				}
				writeJSON(w, 200, map[string]any{"stream": v, "history": s.options.Monitor.StreamHistory(id)})
			} else if parts[1] == "errors" {
				writeJSON(w, 200, map[string]any{"errors": v.Errors})
			} else {
				writeJSON(w, 200, map[string]any{"stream": v})
			}
			return
		default:
			writeJSON(w, 404, map[string]string{"error": "Unknown stream action"})
			return
		}
	}
	switch r.Method {
	case "GET", "HEAD":
		v, err := m.Get(id)
		if err != nil {
			streamError(w, err)
			return
		}
		writeJSON(w, 200, map[string]any{"stream": v})
	case "PUT":
		c, err := decodeChannel(r)
		if err != nil {
			streamError(w, err)
			return
		}
		if c.ID != id {
			streamError(w, errors.New("channel ID in body must match URL"))
			return
		}
		if err = m.Put(r.Context(), c, false); err != nil {
			streamError(w, err)
			return
		}
		v, _ := m.Get(id)
		s.options.Logger.Info("channel updated; receiver replaced", "component", "streams", "channel_id", id)
		writeJSON(w, 200, map[string]any{"stream": v})
	case "DELETE":
		if err := m.Delete(r.Context(), id); err != nil {
			streamError(w, err)
			return
		}
		s.options.Logger.Info("channel deleted", "component", "streams", "channel_id", id)
		w.WriteHeader(204)
	default:
		w.Header().Set("Allow", "GET, HEAD, PUT, DELETE")
		writeJSON(w, 405, map[string]string{"error": "Method not allowed"})
	}
}

// StreamHandler is used only on playback listeners. It exposes no management API.
func (s *Service) StreamHandler() http.Handler { return http.HandlerFunc(s.playback) }
func (s *Service) playback(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	ip, parseErr := netip.ParseAddr(host)
	allowed := false
	if err == nil && parseErr == nil {
		for _, cidr := range s.options.Config.Streaming.AllowedClientNetworks {
			prefix, e := netip.ParsePrefix(cidr)
			if e == nil && prefix.Contains(ip.Unmap()) {
				allowed = true
				break
			}
		}
	}
	if !allowed {
		writeJSON(w, 403, map[string]string{"error": "Playback client IP is not allowed"})
		return
	}
	if r.Method != "GET" {
		w.Header().Set("Allow", "GET")
		writeJSON(w, 405, map[string]string{"error": "Playback requires GET"})
		return
	}
	if r.URL.Path == "/playlist.m3u" {
		if !s.options.Config.Playlist.RemoteEnabled {
			http.NotFound(w, r)
			return
		}
		permitted := false
		for _, cidr := range s.options.Config.Playlist.AllowedClientNetworks {
			p, e := netip.ParsePrefix(cidr)
			if e == nil && p.Contains(ip.Unmap()) {
				permitted = true
			}
		}
		if !permitted {
			writeJSON(w, 403, map[string]string{"error": "Playlist client IP is not allowed"})
			return
		}
		s.exportPlaylist(w, r)
		return
	}
	if s.options.Streams == nil || !s.options.Config.Streaming.Enabled {
		writeJSON(w, 503, map[string]string{"error": "Streaming engine is disabled"})
		return
	}
	id := ""
	switch {
	case strings.HasPrefix(r.URL.Path, "/live/") && strings.HasSuffix(r.URL.Path, ".ts"):
		id = strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/live/"), ".ts")
		if strings.Contains(id, "/") || id == "" {
			writeJSON(w, 400, map[string]string{"error": "Invalid channel path"})
			return
		}
	case strings.HasPrefix(r.URL.Path, "/udp/"):
		target := strings.TrimPrefix(r.URL.Path, "/udp/")
		address, e := netip.ParseAddrPort(target)
		if e != nil || !address.Addr().Is4() || !address.Addr().IsMulticast() || address.Port() == 0 {
			writeJSON(w, 400, map[string]string{"error": "Expected literal IPv4 multicast group and port"})
			return
		}
		id, err = s.options.Streams.FindMulticast(address.Addr().String(), int(address.Port()))
		if err != nil {
			streamError(w, err)
			return
		}
	default:
		writeJSON(w, 404, map[string]string{"error": "Playback endpoint not found"})
		return
	}
	if r.URL.RawQuery != "" {
		writeJSON(w, 400, map[string]string{"error": "Playback query parameters are not supported"})
		return
	}
	sub, err := s.options.Streams.Subscribe(id, ip.Unmap().String())
	if err != nil {
		streamError(w, err)
		return
	}
	defer sub.Close()
	startup, cancel := context.WithTimeout(r.Context(), s.options.Config.Streaming.ConnectTimeout.Value())
	data, err := sub.Next(startup)
	cancel()
	if err != nil {
		if r.Context().Err() == nil {
			writeJSON(w, 503, map[string]string{"error": "No valid MPEG-TS payload before startup deadline or stream stopped"})
		}
		return
	}
	w.Header().Set("Content-Type", "video/mp2t")
	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set("Content-Disposition", "inline")
	rc := http.NewResponseController(w)
	for {
		if err = rc.SetWriteDeadline(time.Now().Add(s.options.Config.Streaming.ClientWriteTimeout.Value())); err != nil && !errors.Is(err, http.ErrNotSupported) {
			return
		}
		n, writeErr := w.Write(data)
		sub.RecordOutput(n)
		if writeErr != nil {
			return
		}
		if err = rc.Flush(); err != nil {
			return
		}
		data, err = sub.Next(r.Context())
		if err != nil {
			return
		}
	}
}

func (s *Service) ServePlayback(ctx context.Context, listener net.Listener) error {
	c := s.options.Config.Management
	server := &http.Server{Handler: s.StreamHandler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: c.ReadTimeout.Value(), IdleTimeout: c.IdleTimeout.Value(), MaxHeaderBytes: 16 << 10, BaseContext: func(net.Listener) context.Context { return ctx }, ErrorLog: slog.NewLogLogger(s.options.Logger.Handler(), slog.LevelError)}
	defer server.Close()
	result := make(chan error, 1)
	// Include idle, unauthenticated and in-flight connections. The manager
	// still enforces the global viewer limit across both playback listeners.
	connectionLimit := s.options.Config.Streaming.MaxClients + playbackConnectionHeadroom
	go func() { result <- server.Serve(netutil.LimitListener(listener, connectionLimit)) }()
	select {
	case err := <-result:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), c.ShutdownTimeout.Value())
		defer cancel()
		err := server.Shutdown(shutdown)
		if err != nil {
			_ = server.Close()
		}
		serveErr := <-result
		if err != nil {
			return err
		}
		if errors.Is(serveErr, http.ErrServerClosed) {
			return nil
		}
		return serveErr
	}
}

// PlaybackURLs uses configured addresses only, rather than untrusted request Host headers.
func PlaybackURLs(address string, port int, id string) string {
	return "http://" + net.JoinHostPort(address, strconv.Itoa(port)) + "/live/" + id + ".ts"
}
