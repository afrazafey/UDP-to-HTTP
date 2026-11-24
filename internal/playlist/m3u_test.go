package playlist

import (
	"github.com/udphttp-proxy/udphttp-proxy/internal/stream"
	"strings"
	"testing"
)

func TestExportRejectsUnsafeStoredLogo(t *testing.T) {
	for _, logo := range []string{"file:///fixture", "data:image/svg+xml,fixture", "https://fixture:password@example.test/logo"} {
		channel := stream.Channel{ID: "fixture", Name: "Fixture", Enabled: true, LogoURL: logo}
		if _, err := Generate([]stream.Channel{channel}, "example.test", 7781, 7782, "", "", ""); err == nil {
			t.Fatal("export accepted an unsafe stored logo")
		}
	}
}

func TestM3UParseSupportedSources(t *testing.T) {
	input := "\ufeff#EXTM3U\n#EXTINF:-1 tvg-id=\"67\" tvg-name=\"TRACE\" tvg-logo=\"http://logos.example/urban.png\" group-title=\"MUSIC\",TRACE URBAN\nhttp://10.14.0.5:7781/udp/239.1.1.61:1234\n#EXTINF:-1,UDP channel\nudp://@239.1.1.62:1234\n#EXTINF:-1,Unicast\nudp://10.14.0.8:1235\n"
	rows, err := Parse(input, "ens18")
	if err != nil || len(rows) != 3 {
		t.Fatal(rows, err)
	}
	c := rows[0].Channel
	if rows[0].Error != "" || c.TVGID != "67" || c.Group != "MUSIC" || c.SourceAddress != "239.1.1.61" || c.SourcePort != 1234 || c.SourceType != "multicast" || c.Interface != "ens18" {
		t.Fatal(rows[0])
	}
	if rows[2].Channel.SourceType != "unicast" {
		t.Fatal(rows[2])
	}
}
func TestM3URejectUnsupportedAndMalformed(t *testing.T) {
	for _, url := range []string{"http://host/live/news.ts", "https://host/arbitrary/239.1.1.1:1234", "http://user:pass@host/udp/239.1.1.1:1234", "http://host/udp/239.1.1.1:1234?token=secret", "http://host/udp/10.0.0.1:1234", "udp://example.com:1234", "udp://239.1.1.1:65536", "file:///etc/passwd", "rtp://239.1.1.1:1234", "udp://239.1.1.1:1234/path"} {
		t.Run(url, func(t *testing.T) {
			rows, err := Parse("#EXTM3U\n#EXTINF:-1,Test\n"+url, "ens18")
			if err != nil || len(rows) != 1 || rows[0].Error == "" {
				t.Fatal(rows, err)
			}
		})
	}
	for _, text := range []string{"", "#EXTINF:-1,Missing header\nudp://239.1.1.1:1234", strings.Repeat("x", MaxBytes+1), string([]byte{255, 254})} {
		if _, err := Parse(text, "ens18"); err == nil {
			t.Fatal("malformed playlist accepted")
		}
	}
	rows, err := Parse("#EXTM3U\n#EXTINF:-1 tvg-id=\"x\" tvg-id=\"y\",Test\nudp://239.1.1.1:1234", "ens18")
	if err != nil || rows[0].Error == "" {
		t.Fatal(rows, err)
	}
}
func TestM3UGenerateMetadataOrderingAndPorts(t *testing.T) {
	channels := []stream.Channel{{ID: "second", Name: "Second", Enabled: true, Group: "MUSIC", SortOrder: 2}, {ID: "trace-urban", Name: "TRACE URBAN", Enabled: true, TVGID: "67", TVGName: "TRACE", LogoURL: "http://logos.example/urban.png", Group: "MUSIC", SortOrder: 1, PreferredPlaybackPort: "secondary"}, {ID: "disabled", Name: "Disabled", Enabled: false}}
	text, err := Generate(channels, "192.168.10.103", 7781, 7782, "preferred", "MUSIC", "")
	if err != nil || !strings.Contains(text, `tvg-id="67"`) || !strings.Contains(text, "192.168.10.103:7782/live/trace-urban.ts") || strings.Contains(text, "disabled") || strings.Index(text, "TRACE URBAN") > strings.Index(text, "Second") {
		t.Fatal(text, err)
	}
	if _, err = Generate(channels, "", 7781, 7782, "primary", "", ""); err == nil {
		t.Fatal("missing advertised host accepted")
	}
	if _, err = Generate(channels, "host", 7781, 0, "secondary", "", ""); err == nil {
		t.Fatal("disabled secondary accepted")
	}
	channels[0].Group = "MUSIC\"\n#injected"
	if _, err = Generate(channels, "host", 7781, 7782, "primary", "", ""); err == nil {
		t.Fatal("metadata injection accepted")
	}
}
