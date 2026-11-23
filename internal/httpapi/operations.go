package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
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
		items, _, err := s.options.Events.QueryEvents(r.Context(), events.Filter{Limit: 8})
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
	writeJSON(w, 200, map[string]any{"version": buildinfo.Version, "commit": buildinfo.Commit, "build_date": buildinfo.Date, "go_version": runtime.Version(), "os": runtime.GOOS, "architecture": runtime.GOARCH, "process_id": os.Getpid(), "started_at": s.started, "uptime_seconds": int64(time.Since(s.started).Seconds()), "goroutines": sample.Goroutines, "resources": sample, "interfaces": items, "configuration_path": path, "database_ready": s.databaseOK(r.Context()), "http_listeners": s.options.Listeners, "dropped_log_events": dropped, "management_policy": "loopback peer + Host + Origin + CSRF on writes; local users are trusted", "remote_authentication": "unavailable; remote binding prohibited", "trusted_proxies": []string{}, "linux_acceptance": "pending"})
}

type settingsRequest struct {
	Revision int64               `json:"revision"`
	Settings operations.Settings `json:"settings"`
	Confirm  bool                `json:"confirm"`
}

func decodeStrict(w http.ResponseWriter, r *http.Request, value any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	var rest any
	if err := decoder.Decode(&rest); err != io.EOF {
		return errors.New("body must contain one JSON object")
	}
	return nil
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
	if err := decodeStrict(w, r, &body); err != nil {
		streamError(w, err)
		return
	}
	if r.URL.Path == "/api/v1/settings/validate" && r.Method == "POST" {
		notes, err := m.Validate(r.Context(), body.Settings)
		if err != nil {
			streamError(w, err)
			return
		}
		writeJSON(w, 200, map[string]any{"valid": true, "notes": notes, "preview": body.Settings, "management_binding_unchanged": true})
		return
	}
	if !body.Confirm {
		streamError(w, errors.New("explicit save confirmation is required"))
		return
	}
	var err error
	if r.Method == "PUT" && r.URL.Path == "/api/v1/settings" {
		_, err = m.Save(r.Context(), body.Revision, body.Settings)
	} else if r.Method == "POST" && r.URL.Path == "/api/v1/settings/rollback" {
		view, e := m.View(r.Context())
		if e != nil {
			err = e
		} else if view["revision"].(int64) != body.Revision {
			err = operations.ErrRevision
		} else {
			_, err = m.Rollback(r.Context())
		}
	} else {
		writeJSON(w, 405, map[string]string{"error": "Unsupported settings method"})
		return
	}
	if err != nil {
		if errors.Is(err, operations.ErrRevision) {
			writeJSON(w, 409, map[string]string{"error": err.Error()})
		} else {
			streamError(w, err)
		}
		return
	}
	s.options.Logger.Info("settings saved", "component", "configuration", "operation", r.URL.Path, "restart", "server settings require process restart; management binding unchanged")
	view, err := m.View(r.Context())
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Saved settings could not be read"})
		return
	}
	writeJSON(w, 200, view)
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
	f := events.Filter{Search: r.URL.Query().Get("search"), Level: r.URL.Query().Get("level"), StreamID: r.URL.Query().Get("stream_id"), Limit: 50}
	if len(f.Search) > 128 || len(f.StreamID) > 64 {
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
