package httpapi

import (
	"errors"
	"io"
	"io/fs"
	"mime"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"regexp"
	"testing"

	"github.com/udphttp-proxy/udphttp-proxy/internal/web"
)

// Use actual embedded production assets and TCP hops, in addition to the
// middleware's malformed-header tests. This is not an Nginx authentication test.
func TestEmbeddedDashboardThroughTrustedProxy(t *testing.T) {
	s := testService()
	enableManagementProxy(s)
	s.options.Assets = web.Assets()
	if _, err := fs.Stat(s.options.Assets, "index.html"); errors.Is(err, fs.ErrNotExist) {
		t.Skip("build frontend to test embedded production assets (CI builds them before Go tests)")
	}
	backend := httptest.NewUnstartedServer(s.Handler())
	s.options.Config.Management.Port = backend.Listener.Addr().(*net.TCPAddr).Port
	backend.Start()
	defer backend.Close()
	target, err := url.Parse(backend.URL)
	if err != nil {
		t.Fatal(err)
	}
	transport := &http.Transport{}
	defer transport.CloseIdleConnections()
	proxy := httptest.NewServer(&httputil.ReverseProxy{Transport: transport, Rewrite: func(p *httputil.ProxyRequest) {
		p.SetURL(target)
		p.Out.Host = target.Host
		p.Out.Header.Set("X-Forwarded-Host", p.In.Host)
		p.Out.Header.Set("X-Forwarded-Proto", "http")
		p.Out.Header.Set("X-Forwarded-For", "192.168.10.102")
		p.Out.Header.Set("X-Forwarded-Port", "80")
	}})
	defer proxy.Close()
	client := proxy.Client()
	defer client.CloseIdleConnections()
	get := func(path, origin string) (int, string, []byte) {
		t.Helper()
		r, err := http.NewRequest("GET", proxy.URL+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		r.Host = "192.168.10.103"
		r.Header.Set("Origin", origin)
		r.Header.Set("Referer", lanOrigin+"/")
		response, err := client.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		body, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		contentType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
		if err != nil {
			t.Fatal(err)
		}
		return response.StatusCode, contentType, body
	}
	code, contentType, html := get("/", lanOrigin)
	if code != 200 || contentType != "text/html" {
		t.Fatalf("embedded HTML: %d %s %s; build frontend before tests", code, contentType, html)
	}
	assets := regexp.MustCompile(`(?:src|href)="(/assets/[^"?]+\.(?:js|css))"`).FindAllSubmatch(html, -1)
	if len(assets) < 2 {
		t.Fatalf("expected embedded JS and CSS in production HTML: %s", html)
	}
	seenJS, seenCSS := false, false
	for _, asset := range assets {
		path := string(asset[1])
		code, contentType, body := get(path, lanOrigin)
		if code != 200 || len(body) == 0 {
			t.Fatalf("%s: %d", path, code)
		}
		if contentType == "text/javascript" || contentType == "application/javascript" {
			seenJS = true
		} else if contentType == "text/css" {
			seenCSS = true
		} else {
			t.Fatalf("%s incorrect MIME type: %s", path, contentType)
		}
	}
	if !seenJS || !seenCSS {
		t.Fatal("both embedded JavaScript and CSS must be served")
	}
	if code, _, _ := get("/api/v1/status", lanOrigin); code != 200 {
		t.Fatal("proxied API status", code)
	}
	if code, _, _ := get("/api/v1/status", "http://evil.example"); code != 403 {
		t.Fatal("cross-origin request accepted", code)
	}
}
