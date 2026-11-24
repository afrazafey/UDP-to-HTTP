// Package playlist parses local text only. It never fetches URLs or authorizes sources.
package playlist

import (
	"bufio"
	"crypto/sha256"
	"errors"
	"fmt"
	"github.com/udphttp-proxy/udphttp-proxy/internal/stream"
	"net"
	"net/netip"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

const MaxBytes = 1 << 20
const MaxEntries = 1000

type Entry struct {
	Attributes map[string]bool `json:"-"`
	Line       int             `json:"line"`
	Channel    stream.Channel  `json:"channel"`
	Error      string          `json:"error"`
	Duplicate  string          `json:"duplicate"`
}

var attribute = regexp.MustCompile(`^([a-zA-Z0-9_-]+)="([^"\r\n]*)"(?:\s+|$)`)
var slug = regexp.MustCompile(`[^a-z0-9_-]+`)
var exportID = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$`)

func source(raw string) (string, string, int, error) {
	if strings.ContainsAny(raw, "\r\n\t ") {
		return "", "", 0, errors.New("source URL contains whitespace")
	}
	raw = strings.Replace(raw, "udp://@", "udp://", 1)
	u, err := url.Parse(raw)
	if err != nil || u.User != nil || u.RawQuery != "" || u.ForceQuery || strings.Contains(raw, "#") {
		return "", "", 0, errors.New("source URL contains credentials, query or fragment")
	}
	address := ""
	switch u.Scheme {
	case "udp":
		if u.Path != "" {
			return "", "", 0, errors.New("UDP URLs cannot contain paths")
		}
		address = u.Host
	case "http", "https":
		if u.Hostname() == "" || !strings.HasPrefix(u.Path, "/udp/") || u.RawPath != "" {
			return "", "", 0, errors.New("only exact legacy HTTP /udp/IPv4:port URLs are supported")
		}
		address = strings.TrimPrefix(u.Path, "/udp/")
		if strings.Contains(address, "/") {
			return "", "", 0, errors.New("unsupported legacy UDP path")
		}
	default:
		return "", "", 0, errors.New("unsupported URL; expected udp://IPv4:port or legacy HTTP /udp/IPv4:port")
	}
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return "", "", 0, errors.New("source requires literal IPv4:port")
	}
	ip, err := netip.ParseAddr(host)
	n, e := strconv.Atoi(port)
	if err != nil || !ip.Is4() || e != nil || n < 1 || n > 65535 || strconv.Itoa(n) != port {
		return "", "", 0, errors.New("invalid IPv4 source or port")
	}
	typ := "unicast"
	if ip.IsMulticast() {
		typ = "multicast"
	}
	if u.Scheme != "udp" && typ != "multicast" {
		return "", "", 0, errors.New("legacy HTTP /udp/ URLs support multicast groups only")
	}
	return typ, ip.String(), n, nil
}
func Parse(text, iface string) ([]Entry, error) {
	if len(text) > MaxBytes || !utf8.ValidString(text) {
		return nil, errors.New("playlist must be valid UTF-8 and at most 1 MiB")
	}
	text = strings.TrimPrefix(text, "\ufeff")
	scanner := bufio.NewScanner(strings.NewReader(text))
	scanner.Buffer(make([]byte, 4096), 64<<10)
	entries := []Entry{}
	line := 0
	header := false
	pending := stream.Channel{}
	metadataErr := ""
	pendingLine := 0
	var attributes map[string]bool
	for scanner.Scan() {
		if len(entries) > MaxEntries {
			return nil, errors.New("playlist exceeds 1000 entries")
		}
		line++
		value := strings.TrimSpace(scanner.Text())
		if value == "" {
			continue
		}
		if !header {
			if value != "#EXTM3U" {
				return nil, errors.New("playlist must begin with #EXTM3U")
			}
			header = true
			continue
		}
		if strings.HasPrefix(value, "#EXTINF:") {
			if pendingLine != 0 {
				entries = append(entries, Entry{Line: pendingLine, Channel: pending, Error: "EXTINF has no source URL"})
			}
			pending = stream.Channel{Interface: iface, Enabled: true, Startup: "on_demand", IdleTimeoutSeconds: 5, PreferredPlaybackPort: "primary"}
			pendingLine = line
			attributes = nil
			metadataErr = ""
			rest := strings.TrimPrefix(value, "#EXTINF:")
			quoted := false
			comma := -1
			for i, r := range rest {
				if r == '"' {
					quoted = !quoted
				}
				if r == ',' && !quoted {
					comma = i
					break
				}
			}
			if comma < 0 {
				metadataErr = "malformed EXTINF metadata"
				continue
			}
			pending.Name = strings.TrimSpace(rest[comma+1:])
			prefix := strings.TrimSpace(rest[:comma])
			durationEnd := strings.IndexAny(prefix, " \t")
			if durationEnd < 0 {
				durationEnd = len(prefix)
			}
			if _, e := strconv.ParseFloat(prefix[:durationEnd], 64); e != nil {
				metadataErr = "invalid EXTINF duration"
			}
			attrs := strings.TrimSpace(prefix[durationEnd:])
			seen := map[string]bool{}
			attributes = seen
			for attrs != "" {
				m := attribute.FindStringSubmatch(attrs)
				if m == nil || seen[m[1]] {
					metadataErr = "malformed or duplicate EXTINF attribute"
					break
				}
				seen[m[1]] = true
				switch m[1] {
				case "tvg-id":
					pending.TVGID = m[2]
				case "tvg-name":
					pending.TVGName = m[2]
				case "tvg-logo":
					pending.LogoURL = m[2]
				case "group-title":
					pending.Group = m[2]
				}
				attrs = strings.TrimSpace(attrs[len(m[0]):])
			}
			continue
		}
		if strings.HasPrefix(value, "#") {
			continue
		}
		if pendingLine == 0 {
			pending = stream.Channel{Interface: iface, Enabled: true, Startup: "on_demand", IdleTimeoutSeconds: 5}
			metadataErr = "source URL has no EXTINF metadata"
			pendingLine = line
		}
		typ, address, port, err := source(value)
		pending.SourceType = typ
		pending.SourceAddress = address
		pending.SourcePort = port
		if err != nil {
			metadataErr = err.Error()
		}
		id := strings.Trim(slug.ReplaceAllString(strings.ToLower(pending.Name), "-"), "-")
		if id == "" {
			id = "channel"
		}
		if len(id) > 48 {
			id = id[:48]
		}
		sum := sha256.Sum256([]byte(value + "/" + iface))
		pending.ID = fmt.Sprintf("%s-%x", id, sum[:4])
		pending.SortOrder = len(entries)
		entries = append(entries, Entry{Line: pendingLine, Channel: pending, Error: metadataErr, Attributes: attributes})
		pendingLine = 0
		attributes = nil
		metadataErr = ""
		pending = stream.Channel{}
		if len(entries) > MaxEntries {
			return nil, errors.New("playlist exceeds 1000 entries")
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if !header {
		return nil, errors.New("empty playlist")
	}
	if pendingLine != 0 {
		entries = append(entries, Entry{Line: pendingLine, Channel: pending, Error: "EXTINF has no source URL"})
	}
	if len(entries) > MaxEntries {
		return nil, errors.New("playlist exceeds 1000 entries")
	}
	return entries, nil
}

func Generate(channels []stream.Channel, host string, primary, secondary int, port, group, id string) (string, error) {
	if host == "" || strings.ContainsAny(host, "/:@\r\n\t ") {
		return "", errors.New("configure playlist.advertised_host without a scheme, port or path")
	}
	u, err := url.Parse("http://" + host)
	if err != nil || u.Hostname() != host {
		return "", errors.New("invalid advertised host")
	}
	if primary < 1 || primary > 65535 || secondary < 0 || secondary > 65535 {
		return "", errors.New("invalid playback ports")
	}
	if port == "" {
		port = "preferred"
	}
	if port != "primary" && port != "secondary" && port != "preferred" {
		return "", errors.New("playlist port must be primary, secondary or preferred")
	}
	if port == "secondary" && secondary == 0 {
		return "", errors.New("secondary playback listener is disabled")
	}
	items := append([]stream.Channel(nil), channels...)
	sort.Slice(items, func(i, j int) bool {
		if items[i].SortOrder != items[j].SortOrder {
			return items[i].SortOrder < items[j].SortOrder
		}
		return items[i].ID < items[j].ID
	})
	var out strings.Builder
	out.WriteString("#EXTM3U\n")
	for _, c := range items {
		if !c.Enabled || (group != "" && c.Group != group) || (id != "" && c.ID != id) {
			continue
		}
		// Defend export even if a legacy database was modified outside this app.
		for _, v := range []string{c.ID, c.Name, c.TVGID, c.TVGName, c.LogoURL, c.Group} {
			if strings.ContainsAny(v, "\r\n\x00") {
				return "", errors.New("unsafe playlist metadata")
			}
		}
		for _, v := range []string{c.TVGID, c.TVGName, c.LogoURL, c.Group} {
			if strings.Contains(v, "\"") {
				return "", errors.New("unsafe playlist attribute")
			}
		}
		if !exportID.MatchString(c.ID) {
			return "", errors.New("unsafe channel ID")
		}
		if err := stream.ValidateLogoURL(c.LogoURL); err != nil {
			return "", err
		}
		selected := primary
		if port == "secondary" || (port == "preferred" && c.PreferredPlaybackPort == "secondary") {
			if secondary == 0 {
				return "", errors.New("channel prefers disabled secondary listener")
			}
			selected = secondary
		}
		fmt.Fprintf(&out, "#EXTINF:-1 tvg-id=\"%s\" tvg-name=\"%s\" tvg-logo=\"%s\" group-title=\"%s\",%s\n#EXTVLCOPT:network-caching=1000\nhttp://%s/live/%s.ts\n", c.TVGID, c.TVGName, c.LogoURL, c.Group, c.Name, net.JoinHostPort(host, strconv.Itoa(selected)), c.ID)
	}
	return out.String(), nil
}
