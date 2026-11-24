package httpapi

import (
	"encoding/json"
	"fmt"
	"github.com/udphttp-proxy/udphttp-proxy/internal/playlist"
	"github.com/udphttp-proxy/udphttp-proxy/internal/stream"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestImportPreviewPreservesExistingReceiverPreferences(t *testing.T) {
	s, _, c := streamingService(t)
	c.Enabled = false
	c.Startup = "always"
	c.IdleTimeoutSeconds = 3
	c.MaxClients = 10
	c.ClientBufferBytes = 4096
	c.UDPReceiveBuffer = 65536
	c.PreferredPlaybackPort = "secondary"
	c.Group = "MUSIC"
	c.TVGID = "67"
	c.LogoURL = "https://logos.example/old.png"
	c.Description = "Preserve description"
	if err := s.options.Streams.Put(t.Context(), c, true); err != nil {
		t.Fatal(err)
	}
	text := fmt.Sprintf("#EXTM3U\n#EXTINF:-1,Updated name\nudp://127.0.0.1:%d\n", c.SourcePort)
	w := managementRequest(t, s, "POST", "/api/v1/import/preview", map[string]any{"text": text, "interface": "127.0.0.1"}, true)
	var result struct {
		Entries []playlist.Entry `json:"entries"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil || w.Code != 200 || len(result.Entries) != 1 {
		t.Fatal(w.Code, w.Body, err)
	}
	got := result.Entries[0]
	if got.Error != "" || got.Duplicate != "existing" || got.Channel.Name != "Updated name" || got.Channel.Enabled || got.Channel.Startup != "always" || got.Channel.MaxClients != 10 || got.Channel.ClientBufferBytes != 4096 || got.Channel.PreferredPlaybackPort != "secondary" || got.Channel.Group != "MUSIC" || got.Channel.TVGID != "67" || got.Channel.Description != c.Description {
		t.Fatal("import reset receiver preferences", got)
	}
}

func TestImportPreviewDoesNotAuthorizeAndCommitIsAtomic(t *testing.T) {
	s := operationsService(t)
	text := "#EXTM3U\n#EXTINF:-1 group-title=\"MUSIC\",TRACE URBAN\nhttp://10.14.0.5:7781/udp/239.1.1.61:1234\n"
	w := managementRequest(t, s, "POST", "/api/v1/import/preview", map[string]any{"text": text, "interface": "127.0.0.1"}, false)
	if w.Code != 403 {
		t.Fatal("import bypassed CSRF")
	}
	w = managementRequest(t, s, "POST", "/api/v1/import/preview", map[string]any{"text": text, "interface": "127.0.0.1"}, true)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	var preview struct {
		Entries []playlist.Entry `json:"entries"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &preview); err != nil || len(preview.Entries) != 1 || !strings.Contains(preview.Entries[0].Error, "allowlist") {
		t.Fatal(w.Body, err)
	}
	good := stream.Channel{ID: "good", Name: "Good", SourceType: "unicast", SourceAddress: "127.0.0.1", SourcePort: 25115, Interface: "127.0.0.1", Enabled: true}
	bad := good
	bad.ID = "bad"
	bad.SourceAddress = "10.14.0.99"
	w = managementRequest(t, s, "POST", "/api/v1/import/commit", map[string]any{"channels": []stream.Channel{good, bad}, "confirm": true}, true)
	if w.Code != 400 || len(s.options.Streams.List()) != 0 {
		t.Fatal("partial or unauthorized import", w.Code, w.Body)
	}
	w = managementRequest(t, s, "POST", "/api/v1/import/commit", map[string]any{"channels": []stream.Channel{good}, "confirm": true}, true)
	if w.Code != 200 || len(s.options.Streams.List()) != 1 {
		t.Fatal(w.Code, w.Body)
	}
	w = managementRequest(t, s, "POST", "/api/v1/import/commit", map[string]any{"channels": []stream.Channel{good}, "confirm": true}, true)
	if w.Code != 409 {
		t.Fatal("existing update lacked confirmation", w.Code)
	}
	w = managementRequest(t, s, "POST", "/api/v1/streams/bulk", map[string]any{"ids": []string{"good"}, "action": "disable", "confirm": true}, true)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	v, _ := s.options.Streams.Get("good")
	if v.Channel.Enabled {
		t.Fatal("disable did not persist in manager")
	}
}
func TestPlaylistExposureAndDiagnosticsRestrictions(t *testing.T) {
	s, _, c := streamingService(t)
	s.options.Config.Playlist.AdvertisedHost = "192.168.10.103"
	c.Group = "MUSIC"
	if err := s.options.Streams.Put(t.Context(), c, true); err != nil {
		t.Fatal(err)
	}
	w := managementRequest(t, s, "GET", "/api/v1/playlist.m3u?group=MUSIC", nil, false)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "192.168.10.103:7781/live/live.ts") || !strings.Contains(w.Header().Get("Content-Type"), "mpegurl") {
		t.Fatal(w.Code, w.Body)
	}
	r := req("GET", "/playlist.m3u")
	r.RemoteAddr = "127.0.0.1:5000"
	w = httptest.NewRecorder()
	s.StreamHandler().ServeHTTP(w, r)
	if w.Code != 404 {
		t.Fatal("remote playlist enabled by default", w.Code)
	}
	s.options.Config.Playlist.RemoteEnabled = true
	s.options.Config.Playlist.AllowedClientNetworks = []string{"192.168.10.0/24"}
	s = New(s.options)
	w = httptest.NewRecorder()
	s.StreamHandler().ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("playlist bypassed independent CIDR", w.Code)
	}
	s.options.Config.Playlist.AllowedClientNetworks = []string{"127.0.0.0/8"}
	s = New(s.options)
	w = httptest.NewRecorder()
	s.StreamHandler().ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	w = managementRequest(t, s, "POST", "/api/v1/diagnostics", map[string]any{"action": "scan", "channel_id": "live"}, true)
	if w.Code != 400 {
		t.Fatal("unrestricted diagnostic accepted", w.Code)
	}
	w = managementRequest(t, s, "POST", "/api/v1/diagnostics", map[string]any{"action": "source", "channel_id": "live", "host": "169.254.169.254"}, true)
	if w.Code != 400 {
		t.Fatal("arbitrary destination accepted", w.Code)
	}
	w = managementRequest(t, s, "GET", "/api/v1/diagnostics", nil, false)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"socket_bound":false`) || !strings.Contains(w.Body.String(), `"multicast_membership_joined":false`) {
		t.Fatal(w.Code, w.Body)
	}
}
