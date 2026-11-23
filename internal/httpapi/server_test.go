package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"testing/fstest"
	"time"

	"github.com/udphttp-proxy/udphttp-proxy/internal/config"
	"github.com/udphttp-proxy/udphttp-proxy/internal/network"
)

type mockStore struct{ err error }

func (s mockStore) Ping(context.Context) error { return s.err }
func testService() *Service {
	return New(Options{Config: config.Default(), Store: mockStore{}, Logger: slog.New(slog.NewJSONHandler(io.Discard, nil)), Discover: func() ([]network.Interface, error) {
		return []network.Interface{{Name: "eth0", IPv4: []string{"10.14.0.5"}}}, nil
	}, Assets: fstest.MapFS{"index.html": {Data: []byte("<h1>Dashboard</h1>")}, "assets/app.js": {Data: []byte("/* application */")}}})
}
func req(method, path string) *http.Request {
	r := httptest.NewRequest(method, "http://127.0.0.1:8080"+path, nil)
	r.RemoteAddr = "127.0.0.1:12345"
	return r
}
func TestEndpoints(t *testing.T) {
	for _, test := range []struct {
		path string
		code int
	}{{"/health", 200}, {"/api/v1/status", 200}, {"/api/v1/interfaces", 200}, {"/", 200}, {"/assets/app.js", 200}, {"/missing", 404}, {"/api/v1/missing", 404}, {"/udp/239.1.1.1:1234", 501}, {"/live/example.ts", 501}} {
		t.Run(test.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			testService().Handler().ServeHTTP(w, req("GET", test.path))
			if w.Code != test.code {
				t.Fatalf("got %d: %s", w.Code, w.Body.String())
			}
			if w.Header().Get("X-Content-Type-Options") != "nosniff" {
				t.Fatal("missing security headers")
			}
		})
	}
}
func TestStatusHasNoFakeMetrics(t *testing.T) {
	w := httptest.NewRecorder()
	testService().Handler().ServeHTTP(w, req("GET", "/api/v1/status"))
	var status map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if status["phase"] != float64(1) || status["components"].(map[string]any)["streaming"] != "unavailable" {
		t.Fatal(status)
	}
	for _, key := range []string{"clients", "bitrate", "streams", "bytes_in", "bytes_out"} {
		if _, ok := status[key]; ok {
			t.Fatalf("fake metric %s", key)
		}
	}
}
func TestSecurity(t *testing.T) {
	for name, change := range map[string]func(*http.Request){
		"remote peer":    func(r *http.Request) { r.RemoteAddr = "192.168.1.5:4567" },
		"forwarded peer": func(r *http.Request) { r.RemoteAddr = "192.168.1.5:4567"; r.Header.Set("X-Forwarded-For", "127.0.0.1") },
		"rebinding host": func(r *http.Request) { r.Host = "attacker.example:8080" },
		"wrong port":     func(r *http.Request) { r.Host = "127.0.0.1:8090" },
		"cross origin":   func(r *http.Request) { r.Header.Set("Origin", "https://attacker.example") },
		"null origin":    func(r *http.Request) { r.Header.Set("Origin", "null") },
		"cross site":     func(r *http.Request) { r.Header.Set("Sec-Fetch-Site", "cross-site") },
	} {
		t.Run(name, func(t *testing.T) {
			r := req("GET", "/api/v1/status")
			change(r)
			w := httptest.NewRecorder()
			testService().Handler().ServeHTTP(w, r)
			if w.Code != 403 {
				t.Fatalf("got %d", w.Code)
			}
		})
	}
}
func TestMethodsAndOrigin(t *testing.T) {
	for _, method := range []string{"POST", "PUT", "DELETE", "OPTIONS"} {
		w := httptest.NewRecorder()
		testService().Handler().ServeHTTP(w, req(method, "/api/v1/status"))
		if w.Code != 405 {
			t.Fatal(w.Code)
		}
	}
	r := req("GET", "/health")
	r.Header.Set("Origin", "http://127.0.0.1:8080")
	w := httptest.NewRecorder()
	testService().Handler().ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
}
func TestUnavailableDependencies(t *testing.T) {
	s := testService()
	s.options.Store = mockStore{errors.New("secret database error")}
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req("GET", "/health"))
	if w.Code != 503 {
		t.Fatal(w.Code)
	}
	s.options.Discover = func() ([]network.Interface, error) { return nil, errors.New("internal failure") }
	w = httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req("GET", "/api/v1/interfaces"))
	if w.Code != 503 {
		t.Fatal(w.Code)
	}
	s.options.Assets = nil
	w = httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req("GET", "/"))
	if w.Code != 503 {
		t.Fatal(w.Code)
	}
}
func TestGracefulShutdown(t *testing.T) {
	l, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := testService()
	s.options.Config.Management.Port = l.Addr().(*net.TCPAddr).Port
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- s.Serve(ctx, l) }()
	client := http.Client{Timeout: 2 * time.Second}
	response, err := client.Get("http://127.0.0.1:" + strconv.Itoa(s.options.Config.Management.Port) + "/health")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 200 {
		t.Fatal(response.StatusCode)
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("server did not shut down")
	}
	if _, err = client.Get(fmt.Sprintf("http://%s/health", l.Addr())); err == nil {
		t.Fatal("listener still accepts connections")
	}
}
