package events

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"
	"unicode/utf8"
)

type Entry struct {
	ID        int64          `json:"id"`
	Time      time.Time      `json:"time"`
	Level     string         `json:"level"`
	Component string         `json:"component"`
	StreamID  string         `json:"stream_id"`
	Message   string         `json:"message"`
	Context   map[string]any `json:"context"`
}
type Filter struct {
	Search, Level, StreamID, Component string
	Since, Until                       time.Time
	Limit, Offset                      int
}
type Repository interface {
	AppendEvent(context.Context, Entry, int, int) error
	QueryEvents(context.Context, Filter) ([]Entry, int, error)
}

var secret = regexp.MustCompile(`(?i)(password|token|secret|authorization|cookie|credential|signature|signed|api[_-]?key|private[_-]?key)`)
var assignment = regexp.MustCompile(`(?i)(password|token|secret|credential|signature|api[_-]?key|private[_-]?key)[[:blank:]]*[:=][[:blank:]]*(?:"[^"\r\n]*"|'[^'\r\n]*'|[^\s,;&]+)`)
var headers = regexp.MustCompile(`(?i)\b(proxy-authorization|authorization|set-cookie|cookie)[[:blank:]]*[:=][^\r\n]*(?:\r?\n[[:blank:]]+[^\r\n]*)*`)
var privateKeys = regexp.MustCompile(`(?s)-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----.*`)
var links = regexp.MustCompile(`https?://[^\s]+`)
var credentials = regexp.MustCompile(`(?i)\b(Bearer|Basic)\s+[^\s,;]+`)

func Redact(value string) string {
	value = privateKeys.ReplaceAllString(value, "[redacted private key]")
	value = headers.ReplaceAllString(value, "${1}=[redacted]")
	value = credentials.ReplaceAllString(value, "${1} [redacted]")
	value = links.ReplaceAllStringFunc(value, func(s string) string {
		u, e := url.Parse(s)
		if e != nil {
			return "[redacted URL]"
		}
		u.RawQuery = ""
		u.Fragment = ""
		u.User = nil
		return u.String()
	})
	value = assignment.ReplaceAllString(value, "${1}=[redacted]")
	value = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return ' '
		}
		return r
	}, value)
	if len(value) > 2048 {
		value = value[:2048]
		for !utf8.ValidString(value) {
			value = value[:len(value)-1]
		}
	}
	return value
}

func Sanitize(e Entry) Entry {
	e.Message = Redact(e.Message)
	e.Component = Redact(e.Component)
	e.StreamID = Redact(e.StreamID)
	e.Level = Redact(e.Level)
	if e.Context != nil {
		e.Context = sanitizeContext(e.Context, 0)
	}
	return e
}

func sanitizeContext(input map[string]any, depth int) map[string]any {
	result := make(map[string]any, min(len(input), 64))
	for key, value := range input {
		if len(result) >= 64 {
			break
		}
		result[Redact(key)] = sanitizeValue(key, value, depth+1)
	}
	return result
}

func sanitizeValue(key string, value any, depth int) any {
	if secret.MatchString(key) {
		return "[redacted]"
	}
	if depth > 8 {
		return "[truncated context]"
	}
	switch item := value.(type) {
	case map[string]any:
		return sanitizeContext(item, depth)
	case []any:
		result := make([]any, min(len(item), 64))
		for i := range result {
			result[i] = sanitizeValue("", item[i], depth+1)
		}
		return result
	case string:
		return Redact(item)
	default:
		return value
	}
}

type Sink struct {
	repo        Repository
	queue       chan Entry
	ctx         context.Context
	cancel      context.CancelFunc
	done        chan struct{}
	stop        chan struct{}
	closed      atomic.Bool
	once        sync.Once
	count, days atomic.Int64
	Dropped     atomic.Uint64
}

func NewSink(repo Repository, count, days int) *Sink {
	ctx, cancel := context.WithCancel(context.Background())
	s := &Sink{repo: repo, queue: make(chan Entry, 256), ctx: ctx, cancel: cancel, done: make(chan struct{}), stop: make(chan struct{})}
	s.SetRetention(count, days)
	go s.run()
	return s
}
func (s *Sink) SetRetention(count, days int) { s.count.Store(int64(count)); s.days.Store(int64(days)) }
func (s *Sink) run() {
	defer close(s.done)
	write := func(e Entry) {
		ctx, cancel := context.WithTimeout(s.ctx, 500*time.Millisecond)
		defer cancel()
		if err := s.repo.AppendEvent(ctx, e, int(s.count.Load()), int(s.days.Load())); err != nil {
			s.Dropped.Add(1)
		}
	}
	for {
		select {
		case <-s.ctx.Done():
			s.Dropped.Add(uint64(len(s.queue)))
			return
		case <-s.stop:
			for {
				select {
				case <-s.ctx.Done():
					s.Dropped.Add(uint64(len(s.queue)))
					return
				case e := <-s.queue:
					write(e)
				default:
					return
				}
			}
		case e := <-s.queue:
			write(e)
		}
	}
}
func (s *Sink) Close() {
	s.once.Do(func() {
		s.closed.Store(true)
		close(s.stop)
		timer := time.NewTimer(2 * time.Second)
		defer timer.Stop()
		select {
		case <-s.done:
		case <-timer.C:
			s.cancel()
			<-s.done
		}
		s.cancel()
	})
}

type Handler struct {
	next  slog.Handler
	sink  *Sink
	attrs []slog.Attr
	group string
}

func NewHandler(next slog.Handler, sink *Sink) *Handler { return &Handler{next: next, sink: sink} }
func (h *Handler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}
func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	copyH := *h
	copyH.attrs = append(append([]slog.Attr{}, h.attrs...), attrs...)
	return &copyH
}
func (h *Handler) WithGroup(name string) slog.Handler {
	copyH := *h
	copyH.group = h.group + name + "."
	return &copyH
}
func (h *Handler) Handle(ctx context.Context, r slog.Record) error {
	e := Entry{Time: r.Time.UTC(), Level: strings.ToLower(r.Level.String()), Component: "service", Message: Redact(r.Message), Context: map[string]any{}}
	if e.Level == "warning" {
		e.Level = "warn"
	}
	clean := slog.NewRecord(r.Time, r.Level, e.Message, r.PC)
	add := func(a slog.Attr) {
		key := h.group + a.Key
		v := a.Value.Resolve()
		var value any
		if secret.MatchString(key) {
			value = "[redacted]"
		} else {
			value = Redact(fmt.Sprint(v.Any()))
		}
		if len(e.Context) < 64 {
			e.Context[key] = value
		}
		if a.Key == "component" {
			e.Component = fmt.Sprint(value)
		}
		if a.Key == "channel_id" || a.Key == "stream_id" {
			e.StreamID = fmt.Sprint(value)
		}
		clean.AddAttrs(slog.Any(key, value))
	}
	for _, a := range h.attrs {
		add(a)
	}
	r.Attrs(func(a slog.Attr) bool { add(a); return true })
	for key, value := range map[string]string{"component": e.Component, "channel_id": e.StreamID, "event_type": "operational", "error_category": "none"} {
		if _, ok := e.Context[key]; !ok {
			if key == "error_category" && r.Level >= slog.LevelWarn {
				value = "operational"
			}
			e.Context[key] = value
			clean.AddAttrs(slog.String(key, value))
		}
	}
	if h.sink.closed.Load() {
		h.sink.Dropped.Add(1)
	} else {
		select {
		case h.sink.queue <- e:
		default:
			h.sink.Dropped.Add(1)
		}
	}
	return h.next.Handle(ctx, clean)
}
