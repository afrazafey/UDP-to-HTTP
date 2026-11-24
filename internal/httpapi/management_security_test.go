package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/udphttp-proxy/udphttp-proxy/internal/config"
	"github.com/udphttp-proxy/udphttp-proxy/internal/operations"
)

const lanOrigin = "http://192.168.10.103"

func enableManagementProxy(s *Service) {
	s.options.Config.Management.ReverseProxy = config.ManagementReverseProxy{Enabled: true, TrustedProxies: []string{"127.0.0.1"}, TrustedOrigins: []string{lanOrigin, "https://dashboard.example:8443"}}
}

func proxiedRequest(method, path string) *http.Request {
	r := req(method, path)
	r.Header.Set("X-Forwarded-Host", "192.168.10.103")
	r.Header.Set("X-Forwarded-Proto", "http")
	r.Header.Set("X-Forwarded-For", "192.168.10.102")
	r.Header.Set("X-Forwarded-Port", "80")
	r.Header.Set("Origin", lanOrigin)
	r.Header.Set("Referer", lanOrigin+"/streams?selected=live")
	r.Header.Set("Sec-Fetch-Site", "same-origin")
	return r
}

func TestManagementProxyAssetsAndGETs(t *testing.T) {
	s := testService()
	enableManagementProxy(s)
	s.options.Assets.(fstest.MapFS)["assets/app.css"] = &fstest.MapFile{Data: []byte("body { color: black }")}
	for path, wantType := range map[string]string{"/": "text/html", "/assets/app.js": "text/javascript", "/assets/app.css": "text/css", "/api/v1/status": "application/json", "/api/v1/csrf": "application/json", "/health": "application/json"} {
		for _, withOrigin := range []bool{true, false} {
			r := proxiedRequest("GET", path)
			if !withOrigin {
				r.Header.Del("Origin")
			}
			w := httptest.NewRecorder()
			s.Handler().ServeHTTP(w, r)
			gotType, _, _ := mime.ParseMediaType(w.Header().Get("Content-Type"))
			if w.Code != 200 || (gotType != wantType && !(wantType == "text/javascript" && gotType == "application/javascript")) {
				t.Errorf("%s origin=%v: %d %q %s", path, withOrigin, w.Code, gotType, w.Body)
			}
			if w.Header().Get("Access-Control-Allow-Origin") != "" || w.Header().Get("X-Content-Type-Options") != "nosniff" {
				t.Fatal("security headers weakened")
			}
		}
	}
}

func TestManagementProxyRequestValidation(t *testing.T) {
	for name, change := range map[string]func(*http.Request){
		"valid":                   func(*http.Request) {},
		"preserved external host": func(r *http.Request) { r.Host = "192.168.10.103" },
		"no browser metadata":     func(r *http.Request) { r.Header.Del("Origin"); r.Header.Del("Referer"); r.Header.Del("Sec-Fetch-Site") },
		"HTTPS nondefault port": func(r *http.Request) {
			r.Host = "dashboard.example:8443"
			r.Header.Set("X-Forwarded-Host", r.Host)
			r.Header.Set("X-Forwarded-Proto", "https")
			r.Header.Set("X-Forwarded-Port", "8443")
			r.Header.Set("Origin", "https://dashboard.example:8443")
			r.Header.Set("Referer", "https://dashboard.example:8443/")
		},
	} {
		t.Run(name, func(t *testing.T) {
			s := testService()
			enableManagementProxy(s)
			r := proxiedRequest("GET", "/api/v1/status")
			change(r)
			w := httptest.NewRecorder()
			s.Handler().ServeHTTP(w, r)
			if w.Code != 200 {
				t.Fatalf("%d %s", w.Code, w.Body)
			}
		})
	}
	for name, change := range map[string]func(*http.Request){
		"untrusted loopback peer": func(r *http.Request) { r.RemoteAddr = "127.0.0.2:1000" },
		"remote peer spoofs loopback": func(r *http.Request) {
			r.RemoteAddr = "192.168.10.102:1000"
			r.Header.Set("X-Forwarded-For", "127.0.0.1")
		},
		"malformed peer":                func(r *http.Request) { r.RemoteAddr = "127.0.0.1" },
		"unapproved origin":             func(r *http.Request) { r.Header.Set("Origin", "http://evil.example") },
		"null origin":                   func(r *http.Request) { r.Header.Set("Origin", "null") },
		"wrong scheme":                  func(r *http.Request) { r.Header.Set("Origin", "https://192.168.10.103") },
		"wrong port":                    func(r *http.Request) { r.Header.Set("Origin", lanOrigin+":81") },
		"explicit default port":         func(r *http.Request) { r.Header.Set("Origin", lanOrigin+":80") },
		"origin path":                   func(r *http.Request) { r.Header.Set("Origin", lanOrigin+"/") },
		"origin credentials":            func(r *http.Request) { r.Header.Set("Origin", "http://user@192.168.10.103") },
		"origin query":                  func(r *http.Request) { r.Header.Set("Origin", lanOrigin+"?x=1") },
		"duplicate origin":              func(r *http.Request) { r.Header.Add("Origin", lanOrigin) },
		"empty origin":                  func(r *http.Request) { r.Header.Set("Origin", "") },
		"multiple origins":              func(r *http.Request) { r.Header.Set("Origin", lanOrigin+" http://evil.example") },
		"different approved origin":     func(r *http.Request) { r.Header.Set("Origin", "https://dashboard.example:8443") },
		"bad referer":                   func(r *http.Request) { r.Header.Set("Referer", "http://evil.example/") },
		"referer without origin":        func(r *http.Request) { r.Header.Del("Origin"); r.Header.Set("Referer", "http://evil.example/") },
		"malformed referer":             func(r *http.Request) { r.Header.Set("Referer", ":invalid") },
		"duplicate referer":             func(r *http.Request) { r.Header.Add("Referer", lanOrigin+"/") },
		"referer credentials":           func(r *http.Request) { r.Header.Set("Referer", "http://user@192.168.10.103/") },
		"cross site":                    func(r *http.Request) { r.Header.Set("Sec-Fetch-Site", "cross-site") },
		"malformed fetch metadata":      func(r *http.Request) { r.Header.Set("Sec-Fetch-Site", "same-origin, cross-site") },
		"duplicate fetch metadata":      func(r *http.Request) { r.Header.Add("Sec-Fetch-Site", "same-origin") },
		"wrong upstream Host":           func(r *http.Request) { r.Host = "evil.example" },
		"wrong upstream port":           func(r *http.Request) { r.Host = "127.0.0.1:8090" },
		"missing forwarded proto":       func(r *http.Request) { r.Header.Del("X-Forwarded-Proto") },
		"missing forwarded client":      func(r *http.Request) { r.Header.Del("X-Forwarded-For") },
		"missing forwarded host":        func(r *http.Request) { r.Header.Del("X-Forwarded-Host") },
		"bad forwarded scheme":          func(r *http.Request) { r.Header.Set("X-Forwarded-Proto", "http, https") },
		"forwarded origin not approved": func(r *http.Request) { r.Header.Set("X-Forwarded-Host", "evil.example") },
		"forwarded host with path":      func(r *http.Request) { r.Header.Set("X-Forwarded-Host", "192.168.10.103/") },
		"duplicate forwarded host":      func(r *http.Request) { r.Header.Add("X-Forwarded-Host", "192.168.10.103") },
		"duplicate forwarded proto":     func(r *http.Request) { r.Header.Add("X-Forwarded-Proto", "http") },
		"duplicate forwarded client":    func(r *http.Request) { r.Header.Add("X-Forwarded-For", "192.168.10.102") },
		"empty forwarded port":          func(r *http.Request) { r.Header.Set("X-Forwarded-Port", "") },
		"inconsistent forwarded port":   func(r *http.Request) { r.Header.Set("X-Forwarded-Port", "443") },
		"forwarded client list":         func(r *http.Request) { r.Header.Set("X-Forwarded-For", "192.168.10.102, 127.0.0.1") },
		"forwarded client with port":    func(r *http.Request) { r.Header.Set("X-Forwarded-For", "192.168.10.102:1234") },
		"forwarded client hostname":     func(r *http.Request) { r.Header.Set("X-Forwarded-For", "localhost") },
		"RFC Forwarded":                 func(r *http.Request) { r.Header.Set("Forwarded", "for=192.168.10.102;host=192.168.10.103;proto=http") },
		"unsupported forwarded header":  func(r *http.Request) { r.Header.Set("X-Forwarded-Prefix", "/") },
		"legacy real IP":                func(r *http.Request) { r.Header.Set("X-Real-IP", "192.168.10.102") },
	} {
		t.Run(name, func(t *testing.T) {
			s := testService()
			enableManagementProxy(s)
			r := proxiedRequest("GET", "/assets/app.js")
			change(r)
			w := httptest.NewRecorder()
			s.Handler().ServeHTTP(w, r)
			if w.Code != 403 {
				t.Fatalf("got %d: %s", w.Code, w.Body)
			}
		})
	}
}

func TestDirectManagementWithProxyEnabledAndDisabled(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		s := testService()
		enableManagementProxy(s)
		s.options.Config.Management.ReverseProxy.Enabled = enabled
		r := req("GET", "/api/v1/status")
		r.Header.Set("Origin", "http://127.0.0.1:8080")
		r.Header.Set("Referer", "http://127.0.0.1:8080/streams")
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatalf("direct enabled=%v: %d", enabled, w.Code)
		}
		r.Header.Set("Origin", lanOrigin)
		w = httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		if w.Code != 403 {
			t.Fatal("approved external origin granted direct access")
		}
		if !enabled {
			w = httptest.NewRecorder()
			s.Handler().ServeHTTP(w, proxiedRequest("GET", "/api/v1/status"))
			if w.Code != 403 {
				t.Fatal("proxy accepted while disabled")
			}
			r = req("GET", "/api/v1/status")
			r.Header.Set("X-Forwarded-Host", "evil.example")
			r.Header.Set("Forwarded", "malformed")
			w = httptest.NewRecorder()
			s.Handler().ServeHTTP(w, r)
			if w.Code != 200 {
				t.Fatal("disabled proxy mode stopped ignoring forwarded headers")
			}
		}
	}
}

func TestExternalHostRequiresProxyTrustOnInitialNavigation(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		s := testService()
		enableManagementProxy(s)
		s.options.Config.Management.ReverseProxy.Enabled = enabled
		for _, forwarded := range []bool{false, true} {
			r := req(http.MethodGet, "/")
			if forwarded {
				r = proxiedRequest(http.MethodGet, "/")
				r.Header.Del("Origin")
				r.Header.Del("Referer")
				r.Header.Del("Sec-Fetch-Site")
			}
			r.Host = "192.168.10.103"
			w := httptest.NewRecorder()
			s.Handler().ServeHTTP(w, r)
			want := http.StatusForbidden
			if enabled && forwarded {
				want = http.StatusOK
			}
			if w.Code != want {
				t.Errorf("proxy enabled=%v, forwarding headers=%v: want %d, got %d: %s", enabled, forwarded, want, w.Code, w.Body)
			}
		}
	}
}

func TestManagementProxyCSRFAndSettingsMutation(t *testing.T) {
	s := operationsService(t, enableManagementProxy)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, proxiedRequest("GET", "/api/v1/csrf"))
	var csrf struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &csrf); err != nil || w.Code != 200 || csrf.Token == "" {
		t.Fatalf("CSRF retrieval: %d %s", w.Code, w.Body)
	}
	w = httptest.NewRecorder()
	s.Handler().ServeHTTP(w, proxiedRequest("GET", "/api/v1/settings"))
	var view struct {
		ReadOnly struct {
			Proxies []string `json:"trusted_proxies"`
			Origins []string `json:"trusted_origins"`
		} `json:"read_only"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &view); err != nil || w.Code != 200 || len(view.ReadOnly.Proxies) != 1 || len(view.ReadOnly.Origins) != 2 {
		t.Fatalf("proxy diagnostics: %d %s", w.Code, w.Body)
	}
	settings := operations.FromConfig(s.options.Config)
	settings.Dashboard.Name = "Managed through Nginx"
	body, err := json.Marshal(map[string]any{"revision": 0, "settings": settings, "confirm": true})
	if err != nil {
		t.Fatal(err)
	}
	for _, token := range []string{"", csrf.Token + "wrong", csrf.Token} {
		r := proxiedRequest("PUT", "/api/v1/settings")
		r.Body = io.NopCloser(bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-CSRF-Token", token)
		w = httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		want := 403
		if token == csrf.Token {
			want = 200
		}
		if w.Code != want {
			t.Fatalf("mutation: want %d, got %d %s", want, w.Code, w.Body)
		}
	}
	if s.options.Settings.Active().Dashboard.Name != settings.Dashboard.Name {
		t.Fatal("authorized mutation did not apply")
	}
	if proxy := s.options.Settings.Active().Management.ReverseProxy; !proxy.Enabled || len(proxy.TrustedProxies) != 1 || len(proxy.TrustedOrigins) != 2 {
		t.Fatal("saved settings replaced YAML-only proxy trust", proxy)
	}
	r := proxiedRequest("PUT", "/api/v1/settings")
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-CSRF-Token", csrf.Token)
	r.Body = io.NopCloser(strings.NewReader(`{"revision":1,"confirm":true,"settings":{"management":{"reverse_proxy":{"enabled":false}}}}`))
	w = httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != 400 {
		t.Fatal("security settings must not be writable through the settings API", w.Code)
	}
	for name, change := range map[string]func(*http.Request){
		"duplicate token":               func(r *http.Request) { r.Header.Add("X-CSRF-Token", csrf.Token) },
		"cross origin with valid token": func(r *http.Request) { r.Header.Set("Origin", "http://evil.example") },
		"bad referer with valid token":  func(r *http.Request) { r.Header.Set("Referer", "http://evil.example/") },
		"spoofed peer with valid token": func(r *http.Request) { r.RemoteAddr = "192.168.10.102:9999" },
	} {
		t.Run(name, func(t *testing.T) {
			r := proxiedRequest("POST", "/api/v1/settings/validate")
			r.Body = io.NopCloser(bytes.NewReader(body))
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("X-CSRF-Token", csrf.Token)
			change(r)
			w := httptest.NewRecorder()
			s.Handler().ServeHTTP(w, r)
			if w.Code != 403 {
				t.Fatalf("%d %s", w.Code, w.Body)
			}
		})
	}
	r = proxiedRequest("POST", "/api/v1/settings/validate")
	r.Body = io.NopCloser(strings.NewReader(string(body)))
	r.Header.Set("X-CSRF-Token", csrf.Token)
	r.Header.Set("Content-Type", "text/plain")
	w = httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != 415 {
		t.Fatal("proxied mutation bypassed JSON requirement", w.Code)
	}
	r = proxiedRequest("POST", "/api/v1/settings/validate")
	r.Body = io.NopCloser(bytes.NewReader(body))
	r.Header.Set("X-CSRF-Token", csrf.Token)
	r.Header.Set("Content-Type", "application/json")
	r.Header.Add("Content-Type", "text/plain")
	w = httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusUnsupportedMediaType {
		t.Fatal("ambiguous media type was accepted", w.Code)
	}
}
