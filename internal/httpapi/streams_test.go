package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/udphttp-proxy/udphttp-proxy/internal/config"
	"github.com/udphttp-proxy/udphttp-proxy/internal/stream"
)

func streamingService(t *testing.T) (*Service, *stream.Manager, stream.Channel) {
	t.Helper()
	cfg := config.Default()
	cfg.Streaming.AllowedClientNetworks = []string{"127.0.0.0/8"}
	cfg.Streaming.AllowedUnicastSources = []string{"127.0.0.1"}
	cfg.Streaming.AllowLoopbackUnicast = true
	cfg.Streaming.ConnectTimeout = config.Duration(200 * time.Millisecond)
	m, err := stream.NewManager(context.Background(), stream.Options{Policy: cfg.StreamPolicy(), Logger: slog.New(slog.NewJSONHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(m.Close)
	l, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	port := l.LocalAddr().(*net.UDPAddr).Port
	l.Close()
	c := stream.Channel{ID: "live", Name: "Live", SourceType: "unicast", SourceAddress: "127.0.0.1", SourcePort: port, Interface: "127.0.0.1", Enabled: true}
	s := New(Options{Config: cfg, Store: mockStore{}, Streams: m, Logger: slog.New(slog.NewJSONHandler(io.Discard, nil))})
	return s, m, c
}
func managementRequest(t *testing.T, s *Service, method, path string, body any, token bool) *httptest.ResponseRecorder {
	t.Helper()
	var data []byte
	if body != nil {
		var err error
		data, err = json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
	}
	r := req(method, path)
	r.Body = io.NopCloser(bytes.NewReader(data))
	r.Header.Set("Content-Type", "application/json")
	if token {
		r.Header.Set("X-CSRF-Token", s.csrf)
	}
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	return w
}
func TestStreamManagementAndCSRF(t *testing.T) {
	s, _, c := streamingService(t)
	if w := managementRequest(t, s, "POST", "/api/v1/streams", c, false); w.Code != 403 {
		t.Fatalf("CSRF accepted: %d", w.Code)
	}
	if w := managementRequest(t, s, "POST", "/api/v1/streams", c, true); w.Code != 201 {
		t.Fatalf("create %d %s", w.Code, w.Body)
	}
	if w := managementRequest(t, s, "POST", "/api/v1/streams", c, true); w.Code != 409 {
		t.Fatalf("duplicate %d", w.Code)
	}
	for _, path := range []string{"/api/v1/streams", "/api/v1/streams/live", "/api/v1/streams/live/status", "/api/v1/streams/live/errors", "/api/v1/clients", "/api/v1/csrf"} {
		if w := managementRequest(t, s, "GET", path, nil, false); w.Code != 200 {
			t.Fatalf("%s: %d", path, w.Code)
		}
	}
	c.Name = "Updated"
	if w := managementRequest(t, s, "PUT", "/api/v1/streams/live", c, true); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	for _, action := range []string{"start", "stop", "restart", "stop"} {
		if w := managementRequest(t, s, "POST", "/api/v1/streams/live/"+action, nil, true); w.Code != 202 {
			t.Fatalf("%s %d %s", action, w.Code, w.Body)
		}
	}
	if w := managementRequest(t, s, "DELETE", "/api/v1/streams/live", nil, true); w.Code != 204 {
		t.Fatal(w.Code)
	}
	if w := managementRequest(t, s, "GET", "/api/v1/streams/live", nil, false); w.Code != 404 {
		t.Fatal(w.Code)
	}
}
func TestStreamManagementSecurityAndValidation(t *testing.T) {
	s, _, c := streamingService(t)
	c.SourceAddress = "169.254.169.254"
	if w := managementRequest(t, s, "POST", "/api/v1/streams", c, true); w.Code != 400 {
		t.Fatal(w.Code)
	}
	r := req("POST", "/api/v1/streams")
	r.Header.Set("X-CSRF-Token", s.csrf)
	r.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != 415 {
		t.Fatal(w.Code)
	}
	r = req("POST", "/api/v1/streams")
	r.RemoteAddr = "10.0.0.5:1234"
	r.Header.Set("X-CSRF-Token", s.csrf)
	w = httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("remote mutation accepted")
	}
	r = req("POST", "/api/v1/streams")
	r.Header.Set("X-CSRF-Token", s.csrf)
	r.Header.Set("Content-Type", "application/json")
	r.Body = io.NopCloser(strings.NewReader(`{"unexpected":true}`))
	w = httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != 400 {
		t.Fatal("unknown field accepted")
	}
}
func TestPlaybackPolicyAndPaths(t *testing.T) {
	s, _, _ := streamingService(t)
	for _, test := range []struct {
		path string
		code int
	}{{"/api/v1/status", 404}, {"/live/missing.ts", 404}, {"/udp/example.com:1234", 400}, {"/udp/127.0.0.1:1234", 400}, {"/udp/239.1.1.1:1234", 404}, {"/live/missing.ts?source=127.0.0.1", 400}} {
		w := httptest.NewRecorder()
		s.StreamHandler().ServeHTTP(w, req("GET", test.path))
		if w.Code != test.code {
			t.Fatalf("%s: %d %s", test.path, w.Code, w.Body)
		}
	}
	r := req("GET", "/live/live.ts")
	r.RemoteAddr = "10.0.0.5:1234"
	r.Header.Set("X-Forwarded-For", "127.0.0.1")
	w := httptest.NewRecorder()
	s.StreamHandler().ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("CIDR bypass")
	}
	s.options.Config.Streaming.AllowedClientNetworks = nil
	w = httptest.NewRecorder()
	s.StreamHandler().ServeHTTP(w, req("GET", "/live/live.ts"))
	if w.Code != 403 {
		t.Fatal("empty allowlist must deny")
	}
}
func TestStartupInactivityReturns503(t *testing.T) {
	s, m, c := streamingService(t)
	if err := m.Put(context.Background(), c, true); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	s.StreamHandler().ServeHTTP(w, req("GET", "/live/live.ts"))
	if w.Code != 503 {
		t.Fatal(w.Code)
	}
	if len(m.Clients()) != 0 {
		t.Fatal("startup timeout leaked client")
	}
}
func fixtureTS() []byte {
	data := make([]byte, 7*188)
	for i := 0; i < 7; i++ {
		p := data[i*188:]
		p[0] = 0x47
		p[1] = 1
		p[2] = 0
		p[3] = 0x10 | byte(i)
	}
	return data
}
func TestSimultaneousHTTPViewers(t *testing.T) {
	s, m, c := streamingService(t)
	s.options.Config.Streaming.ConnectTimeout = config.Duration(2 * time.Second)
	if err := m.Put(context.Background(), c, true); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(s.StreamHandler())
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	responses := make(chan *http.Response, 2)
	failures := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() {
			r, _ := http.NewRequestWithContext(ctx, "GET", server.URL+"/live/live.ts", nil)
			response, err := http.DefaultClient.Do(r)
			if err != nil {
				failures <- err
				return
			}
			responses <- response
		}()
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		v, _ := m.Get(c.ID)
		if v.Clients == 2 && v.State == "waiting" {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	conn, err := net.DialUDP("udp4", nil, &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: c.SourcePort})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	want := fixtureTS()
	if _, err = conn.Write(want); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		select {
		case response := <-responses:
			if response.StatusCode != 200 || response.Header.Get("Content-Type") != "video/mp2t" {
				response.Body.Close()
				t.Fatalf("invalid playback response: %d", response.StatusCode)
			}
			got := make([]byte, len(want))
			_, err = io.ReadFull(response.Body, got)
			response.Body.Close()
			if err != nil || !bytes.Equal(got, want) {
				t.Fatalf("TS mismatch: %v", err)
			}
		case err := <-failures:
			t.Fatal(err)
		case <-time.After(3 * time.Second):
			t.Fatal("HTTP viewer stuck")
		}
	}
	cancel()
	deadline = time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if len(m.Clients()) == 0 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("HTTP cancellation leaked clients")
}
