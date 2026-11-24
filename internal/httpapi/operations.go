package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/netip"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/udphttp-proxy/udphttp-proxy/internal/buildinfo"
	"github.com/udphttp-proxy/udphttp-proxy/internal/events"
	"github.com/udphttp-proxy/udphttp-proxy/internal/operations"
)

func unavailable(w http.ResponseWriter) {
	writeJSON(w, 503, map[string]string{"error": "Operations capability is unavailable"})
}
func (s *Service) operationsDashboard(w http.ResponseWriter, r *http.Request) {
	if s.options.Monitor == nil {
		unavailable(w)
		return
	}
	sample, history, summary := s.options.Monitor.View()
	recent := []events.Entry{}
	if s.options.Events != nil {
		items, _, err := s.options.Events.QueryEvents(r.Context(), s.retainedFilter(events.Filter{Limit: 8}))
		if err != nil {
			writeJSON(w, 503, map[string]string{"error": "Activity history is unavailable"})
			return
		}
		recent = items
	}
	writeJSON(w, 200, map[string]any{"summary": summary, "sample": sample, "history": history, "activity": recent, "database_ready": s.databaseOK(r.Context()), "version": buildinfo.Version, "uptime_seconds": int64(time.Since(s.started).Seconds()), "sample_interval_seconds": 3, "history_limit": 240})
}
func (s *Service) metrics(w http.ResponseWriter, r *http.Request) {
	if s.options.Monitor == nil {
		unavailable(w)
		return
	}
	sample, history, summary := s.options.Monitor.View()
	writeJSON(w, 200, map[string]any{"sample": sample, "history": history, "summary": summary, "sample_interval_seconds": 3, "history_limit": 240})
}
func (s *Service) systemInfo(w http.ResponseWriter, r *http.Request) {
	if s.options.Monitor == nil {
		unavailable(w)
		return
	}
	sample, _, _ := s.options.Monitor.View()
	items, err := s.options.Discover()
	if err != nil {
		writeJSON(w, 503, map[string]string{"error": "Interface discovery unavailable"})
		return
	}
	path := s.options.ConfigPath
	if path == "" {
		path = "built-in defaults"
	}
	dropped := uint64(0)
	if s.options.DroppedLogs != nil {
		dropped = s.options.DroppedLogs()
	}
	authentication := "unavailable; remote binding prohibited"
	proxies := []string{}
	if s.options.Config.Management.ReverseProxy.Enabled {
		authentication = "enforced by configured reverse proxy; native remote binding prohibited"
		proxies = append(proxies, s.options.Config.Management.ReverseProxy.TrustedProxies...)
	}
	var schema *int
	if store, ok := s.options.Store.(interface {
		SchemaVersion(context.Context) (int, error)
	}); ok {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		if version, err := store.SchemaVersion(ctx); err == nil {
			schema = &version
		}
		cancel()
	}
	writeJSON(w, 200, map[string]any{"version": buildinfo.Version, "commit": buildinfo.Commit, "build_date": buildinfo.Date, "go_version": runtime.Version(), "os": runtime.GOOS, "architecture": runtime.GOARCH, "process_id": os.Getpid(), "started_at": s.started, "uptime_seconds": int64(time.Since(s.started).Seconds()), "goroutines": sample.Goroutines, "resources": sample, "interfaces": items, "configuration_path": path, "database_ready": s.databaseOK(r.Context()), "database_schema": schema, "http_listeners": s.options.Listeners, "dropped_log_events": dropped, "management_policy": "loopback peer + direct/trusted-proxy Host + Origin/Referer + fetch metadata + CSRF on writes; local users are trusted", "remote_authentication": authentication, "trusted_proxies": proxies, "reverse_proxy": s.options.Config.Management.ReverseProxy, "linux_acceptance": "pending", "application": buildinfo.Name, "developer": buildinfo.Developer, "website": buildinfo.Website, "environment": buildinfo.Environment(s.options.Environment), "release_maturity": buildinfo.Maturity()})
}

type settingsRequest struct {
	Revision            int64               `json:"revision"`
	Settings            operations.Settings `json:"settings"`
	Confirm             bool                `json:"confirm"`
	ConfirmAllMulticast bool                `json:"confirm_all_multicast"`
}

func (s *Service) settings(w http.ResponseWriter, r *http.Request) {
	m := s.options.Settings
	if m == nil {
		unavailable(w)
		return
	}
	if r.Method == "GET" || r.Method == "HEAD" {
		if r.URL.Path != "/api/v1/settings" {
			writeJSON(w, 405, map[string]string{"error": "Use POST for this operation"})
			return
		}
		view, err := m.View(r.Context())
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "Settings storage unavailable"})
			return
		}
		writeJSON(w, 200, view)
		return
	}
	var body settingsRequest
	if err := decodeStrict(r, &body); err != nil {
		streamError(w, err)
		return
	}
	if r.URL.Path == "/api/v1/settings/validate" && r.Method == "POST" {
		preview, notes, confirmAll, err := m.Preview(r.Context(), body.Settings)
		if err != nil {
			streamError(w, err)
			return
		}
		writeJSON(w, 200, map[string]any{"valid": true, "notes": notes, "preview": preview, "management_binding_unchanged": true, "requires_all_multicast_confirmation": confirmAll})
		return
	}
	if !body.Confirm {
		streamError(w, errors.New("explicit save confirmation is required"))
		return
	}
	var err error
	if r.Method == "PUT" && r.URL.Path == "/api/v1/settings" {
		_, err = m.Save(r.Context(), body.Revision, body.Settings, body.ConfirmAllMulticast)
	} else if r.Method == "POST" && r.URL.Path == "/api/v1/settings/rollback" {
		_, err = m.Rollback(r.Context(), body.Revision)
	} else {
		writeJSON(w, 405, map[string]string{"error": "Unsupported settings method"})
		return
	}
	if err != nil {
		if errors.Is(err, operations.ErrPersistence) {
			s.options.Logger.Error("settings persistence failed", "component", "configuration", "error", err)
			writeJSON(w, 500, map[string]string{"error": "Settings storage could not confirm the save; reload the current revision before retrying"})
		} else if errors.Is(err, operations.ErrRevision) {
			writeJSON(w, 409, map[string]string{"error": err.Error()})
		} else {
			streamError(w, err)
		}
		return
	}
	s.options.Logger.Info("settings saved", "component", "configuration", "operation", r.URL.Path)
	view, err := m.View(r.Context())
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Saved settings could not be read"})
		return
	}
	writeJSON(w, 200, view)
}

func (s *Service) testNetworkPolicy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		writeJSON(w, 405, map[string]string{"error": "Policy tests require POST"})
		return
	}
	if s.options.Settings == nil {
		unavailable(w)
		return
	}
	var body struct {
		Address string               `json:"address"`
		Kind    string               `json:"kind"`
		Draft   *operations.Settings `json:"settings"`
	}
	if err := decodeStrict(r, &body); err != nil {
		streamError(w, err)
		return
	}
	ip, err := netip.ParseAddr(strings.TrimSpace(body.Address))
	if err != nil || ip.Zone() != "" {
		streamError(w, errors.New("address must be a literal IP address without a zone"))
		return
	}
	policy, err := s.options.Settings.Policy(body.Draft)
	if err != nil {
		streamError(w, err)
		return
	}
	switch body.Kind {
	case "playback":
		writeJSON(w, 200, policy.Playback.Check(ip))
	case "playlist":
		allowed := policy.Playback.Check(ip)
		if allowed.Allowed {
			if !s.options.Config.Playlist.RemoteEnabled {
				allowed.Allowed, allowed.Reason, allowed.Matched = false, "Remote playlists are disabled", ""
			} else {
				allowed = policy.Playlist.Check(ip)
			}
		}
		writeJSON(w, 200, allowed)
	case "multicast", "unicast":
		writeJSON(w, 200, policy.Sources.Check(body.Kind, ip))
	default:
		streamError(w, errors.New("kind must be playback, playlist, multicast or unicast"))
	}
}
func (s *Service) clientAction(w http.ResponseWriter, r *http.Request) {
	if s.options.Streams == nil {
		unavailable(w)
		return
	}
	if r.Method != "DELETE" {
		writeJSON(w, 405, map[string]string{"error": "Use DELETE to disconnect a client"})
		return
	}
	id, err := strconv.ParseUint(strings.TrimPrefix(r.URL.Path, "/api/v1/clients/"), 10, 64)
	if err != nil {
		streamError(w, errors.New("invalid client ID"))
		return
	}
	if err = s.options.Streams.Disconnect(id); err != nil {
		streamError(w, err)
		return
	}
	s.options.Logger.Info("playback client disconnect requested", "component", "http", "client_id", id)
	w.WriteHeader(204)
}
func logFilter(r *http.Request) (events.Filter, error) {
	f := events.Filter{Search: r.URL.Query().Get("search"), Level: r.URL.Query().Get("level"), StreamID: r.URL.Query().Get("stream_id"), Component: r.URL.Query().Get("component"), Limit: 50}
	if len(f.Search) > 128 || len(f.StreamID) > 64 || len(f.Component) > 64 {
		return f, errors.New("log search is too long")
	}
	if f.Level != "" && f.Level != "debug" && f.Level != "info" && f.Level != "warn" && f.Level != "error" {
		return f, errors.New("invalid log level")
	}
	var err error
	if v := r.URL.Query().Get("limit"); v != "" {
		f.Limit, err = strconv.Atoi(v)
		if err != nil {
			return f, err
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		f.Offset, err = strconv.Atoi(v)
		if err != nil {
			return f, err
		}
	}
	maxLimit := 100
	if strings.HasSuffix(r.URL.Path, "/export") {
		maxLimit = 1000
		f.Limit = 1000
		f.Offset = 0
	}
	if f.Limit < 1 || f.Limit > maxLimit || f.Offset < 0 || f.Offset > 10000 {
		return f, errors.New("invalid log pagination")
	}
	if v := r.URL.Query().Get("since"); v != "" {
		f.Since, err = time.Parse(time.RFC3339, v)
		if err != nil {
			return f, errors.New("invalid since timestamp")
		}
	}
	if v := r.URL.Query().Get("until"); v != "" {
		f.Until, err = time.Parse(time.RFC3339, v)
		if err != nil {
			return f, errors.New("invalid until timestamp")
		}
	}
	if !f.Since.IsZero() && !f.Until.IsZero() && f.Since.After(f.Until) {
		return f, errors.New("since must precede until")
	}
	return f, nil
}
func (s *Service) logs(w http.ResponseWriter, r *http.Request) {
	if s.options.Events == nil {
		unavailable(w)
		return
	}
	filter, err := logFilter(r)
	if err != nil {
		streamError(w, err)
		return
	}
	filter = s.retainedFilter(filter)
	items, total, err := s.options.Events.QueryEvents(r.Context(), filter)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Log storage unavailable"})
		return
	}
	if strings.HasSuffix(r.URL.Path, "/export") {
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.Header().Set("Content-Disposition", `attachment; filename="udphttp-diagnostics.ndjson"`)
		encoder := json.NewEncoder(w)
		for _, e := range items {
			if err = encoder.Encode(e); err != nil {
				return
			}
		}
		return
	}
	writeJSON(w, 200, map[string]any{"logs": items, "total": total, "limit": filter.Limit, "offset": filter.Offset})
}
func (s *Service) retainedFilter(f events.Filter) events.Filter {
	days := s.options.Config.Logging.RetentionDays
	if s.options.Settings != nil {
		days = s.options.Settings.Active().Logging.RetentionDays
	}
	if days > 0 {
		oldest := time.Now().UTC().AddDate(0, 0, -days)
		if f.Since.IsZero() || f.Since.Before(oldest) {
			f.Since = oldest
		}
	}
	return f
}
