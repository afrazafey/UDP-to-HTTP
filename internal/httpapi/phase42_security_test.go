package httpapi

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/udphttp-proxy/udphttp-proxy/internal/config"
)

func TestManagementRejectsCrossOriginFetchMetadata(t *testing.T) {
	service := New(Options{Config: config.Default()})
	for _, site := range []string{"same-site", "cross-site"} {
		r := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8080/api/v1/csrf", nil)
		r.RemoteAddr = "127.0.0.1:40000"
		r.Header.Set("Sec-Fetch-Site", site)
		w := httptest.NewRecorder()
		service.Handler().ServeHTTP(w, r)
		if w.Code != http.StatusForbidden || w.Header().Get("Access-Control-Allow-Origin") != "" {
			t.Fatalf("%s: %d", site, w.Code)
		}
		if !strings.Contains(w.Header().Get("Content-Security-Policy"), "frame-ancestors 'none'") ||
			!strings.Contains(w.Header().Get("Content-Security-Policy"), "form-action 'self'") ||
			!strings.Contains(w.Header().Get("Permissions-Policy"), "camera=()") {
			t.Fatal("restricted management headers are required on errors")
		}
	}
}

func TestStrictManagementJSON(t *testing.T) {
	for name, body := range map[string]string{
		"null object":              "null",
		"duplicate field":          `{"confirm":true,"confirm":false}`,
		"alternate field spelling": `{"confirm":true,"CONFIRM":false}`,
		"duplicate nested field":   `{"values":[{"item":1,"item":2}]}`,
		"extra document":           `{}{}`,
		"unknown property":         `{"other":true}`,
	} {
		t.Run(name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/api/v1/settings", strings.NewReader(body))
			var value struct {
				Confirm bool             `json:"confirm"`
				Values  []map[string]int `json:"values"`
			}
			if err := decodeStrict(r, &value); err == nil {
				t.Fatal("ambiguous request was accepted")
			}
		})
	}
	r := httptest.NewRequest(http.MethodPost, "/api/v1/settings", nil)
	r.Body = http.MaxBytesReader(httptest.NewRecorder(),
		io.NopCloser(strings.NewReader("{}"+strings.Repeat(" ", 1<<20))), 1<<20)
	w := httptest.NewRecorder()
	var value settingsRequest
	streamError(w, decodeStrict(r, &value))
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized chunked JSON returned %d", w.Code)
	}
}
