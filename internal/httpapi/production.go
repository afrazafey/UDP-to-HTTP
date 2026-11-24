package httpapi

import (
	"errors"
	"github.com/udphttp-proxy/udphttp-proxy/internal/playlist"
	"github.com/udphttp-proxy/udphttp-proxy/internal/stream"
	"io"
	"net"
	"net/http"
	"net/netip"
	"sort"
	"time"
)

func (s *Service) exportPlaylist(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" && r.Method != "HEAD" {
		writeJSON(w, 405, map[string]string{"error": "playlist requires GET"})
		return
	}
	if s.options.Streams == nil {
		unavailable(w)
		return
	}
	channels := []stream.Channel{}
	for _, v := range s.options.Streams.List() {
		channels = append(channels, v.Channel)
	}
	cfg := s.options.Config
	text, err := playlist.Generate(channels, cfg.Playlist.AdvertisedHost, cfg.Network.HTTPPort, cfg.Network.SecondaryHTTPPort, r.URL.Query().Get("port"), r.URL.Query().Get("group"), r.URL.Query().Get("id"))
	if err != nil {
		streamError(w, err)
		return
	}
	w.Header().Set("Content-Type", "audio/x-mpegurl; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="udphttp-channels.m3u"`)
	s.options.Logger.Info("playlist exported", "component", "playlist", "event_type", "playlist_export", "channels", len(channels))
	if r.Method != "HEAD" {
		_, _ = w.Write([]byte(text))
	}
}
func (s *Service) playlistConfiguration(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" && r.Method != "HEAD" {
		writeJSON(w, 405, map[string]string{"error": "read-only playlist configuration"})
		return
	}
	writeJSON(w, 200, map[string]any{"playlist": s.options.Config.Playlist, "primary_port": s.options.Config.Network.HTTPPort, "secondary_port": s.options.Config.Network.SecondaryHTTPPort})
}
func (s *Service) channelImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		writeJSON(w, 405, map[string]string{"error": "import requires POST"})
		return
	}
	if s.options.Streams == nil {
		unavailable(w)
		return
	}
	if r.URL.Path == "/api/v1/import/preview" {
		var body struct {
			Text      string `json:"text"`
			Interface string `json:"interface"`
		}
		if err := decodeStrict(r, &body); err != nil {
			streamError(w, err)
			return
		}
		if body.Interface == "" {
			body.Interface = s.options.Config.Network.MulticastInterface
		}
		entries, err := playlist.Parse(body.Text, body.Interface)
		if err != nil {
			streamError(w, err)
			return
		}
		existing := map[string]stream.Channel{}
		for _, item := range s.options.Streams.List() {
			existing[item.Channel.SourceKey()] = item.Channel
		}
		seen := map[string]bool{}
		cfg := s.options.Config
		sources := s.options.Streams.Sources()
		for i := range entries {
			row := &entries[i]
			row.Channel = row.Channel.Defaults(cfg.Streaming.MaxClients, cfg.Streaming.ClientBufferBytes, cfg.Streaming.UDPReceiveBuffer)
			row.Duplicate = "new"
			if row.Error != "" {
				continue
			}
			key := row.Channel.SourceKey()
			if seen[key] {
				row.Duplicate = "within_import"
				row.Error = "duplicate source in imported playlist"
			}
			seen[key] = true
			if c, ok := existing[key]; ok {
				row.Duplicate = "existing"
				incoming := row.Channel
				// A playlist carries metadata, not receiver tuning or enabled state.
				// Preserve existing runtime preferences and absent metadata fields.
				c.Name = incoming.Name
				c.SortOrder = incoming.SortOrder
				if row.Attributes["tvg-id"] {
					c.TVGID = incoming.TVGID
				}
				if row.Attributes["tvg-name"] {
					c.TVGName = incoming.TVGName
				}
				if row.Attributes["tvg-logo"] {
					c.LogoURL = incoming.LogoURL
				}
				if row.Attributes["group-title"] {
					c.Group = incoming.Group
				}
				row.Channel = c
			}
			if err := row.Channel.ValidateWithSources(sources); err != nil {
				row.Error = err.Error()
			}
			if row.Error == "" {
				if err := stream.ValidateInterface(row.Channel); err != nil {
					row.Error = err.Error()
				}
			}
		}
		s.options.Logger.Info("playlist import previewed", "component", "import", "event_type", "import_preview", "rows", len(entries))
		writeJSON(w, 200, map[string]any{"entries": entries, "atomic": true, "sources_automatically_authorized": false})
		return
	}
	if r.URL.Path != "/api/v1/import/commit" {
		http.NotFound(w, r)
		return
	}
	var body struct {
		Channels       []stream.Channel `json:"channels"`
		UpdateExisting bool             `json:"update_existing"`
		Confirm        bool             `json:"confirm"`
	}
	if err := decodeStrict(r, &body); err != nil {
		streamError(w, err)
		return
	}
	if !body.Confirm || len(body.Channels) == 0 || len(body.Channels) > playlist.MaxEntries {
		streamError(w, errors.New("confirmed import requires 1..1000 selected channels"))
		return
	}
	if err := s.options.Streams.ImportChannels(r.Context(), body.Channels, body.UpdateExisting); err != nil {
		streamError(w, err)
		return
	}
	s.options.Logger.Info("playlist import committed", "component", "import", "event_type", "import_commit", "channels", len(body.Channels))
	writeJSON(w, 200, map[string]any{"imported": len(body.Channels), "atomic": true})
}
func (s *Service) bulkChannels(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		writeJSON(w, 405, map[string]string{"error": "bulk actions require POST"})
		return
	}
	if s.options.Streams == nil {
		unavailable(w)
		return
	}
	var body struct {
		IDs     []string `json:"ids"`
		Action  string   `json:"action"`
		Confirm bool     `json:"confirm"`
	}
	if err := decodeStrict(r, &body); err != nil {
		streamError(w, err)
		return
	}
	if !body.Confirm || len(body.IDs) == 0 || len(body.IDs) > s.options.Config.Streaming.MaxStreams {
		streamError(w, errors.New("select channels and explicitly confirm bulk action"))
		return
	}
	if body.Action != "enable" && body.Action != "disable" && body.Action != "delete" {
		streamError(w, errors.New("bulk action must be enable, disable or delete"))
		return
	}
	if err := s.options.Streams.BulkChannels(r.Context(), body.IDs, body.Action); err != nil {
		streamError(w, err)
		return
	}
	s.options.Logger.Info("bulk channels updated", "component", "channels", "event_type", "bulk_"+body.Action, "count", len(body.IDs))
	writeJSON(w, 200, map[string]any{"affected": len(body.IDs), "atomic": true})
}

func (s *Service) diagnostics(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" && r.Method != "HEAD" && r.Method != "POST" {
		writeJSON(w, 405, map[string]string{"error": "diagnostics require GET or POST"})
		return
	}
	if r.Method == "POST" {
		var body struct {
			Action    string `json:"action"`
			ChannelID string `json:"channel_id"`
		}
		if err := decodeStrict(r, &body); err != nil {
			streamError(w, err)
			return
		}
		s.diagnosticAction(w, r, body.Action, body.ChannelID)
		return
	}
	items, err := s.options.Discover()
	if err != nil {
		streamError(w, err)
		return
	}
	observations := []map[string]any{}
	groups := []string{}
	if s.options.Streams != nil {
		for _, v := range s.options.Streams.List() {
			valid := v.Channel.ValidateWithSources(s.options.Streams.Sources()) == nil
			if v.Channel.SourceType == "multicast" {
				groups = append(groups, v.Channel.SourceAddress)
			}
			observations = append(observations, map[string]any{"channel_id": v.Channel.ID, "configuration_valid": valid, "interface_error": interfaceError(v.Channel), "socket_bound": v.SocketBound, "multicast_membership_joined": v.MembershipJoined, "membership_observation": "application-owned socket join, not all host memberships", "last_datagram_at": v.LastDatagramAt, "last_valid_ts_at": v.LastPacketAt, "mpeg_ts_packets": v.Counters.Packets, "http_payload_bytes_delivered": v.Counters.OutputBytes, "active_viewers": v.Clients})
		}
	}
	sort.Strings(groups)
	clients := []stream.ClientSnapshot{}
	if s.options.Streams != nil {
		clients = s.options.Streams.Clients()
	}
	writeJSON(w, 200, map[string]any{
		"interfaces": items, "configured_udp_interface": s.options.Config.Network.MulticastInterface, "configured_http_interface": s.options.Config.Network.HTTPInterface,
		"listeners": s.options.Listeners, "groups": groups, "channels": observations,
		"interface_error_counters": "see per-interface error_counters; null when unsupported",
		"route_selection":          "explicit receiver interface; OS multicast route table is not inspected", "http_clients": clients,
	})
}
func interfaceError(c stream.Channel) any {
	if err := stream.ValidateInterface(c); err != nil {
		return err.Error()
	}
	return nil
}
func (s *Service) diagnosticAction(w http.ResponseWriter, r *http.Request, action, id string) {
	if action == "listener_connectivity" || action == "playback_ports" {
		results := []map[string]any{}
		for _, l := range s.options.Listeners {
			if l.Kind != "playback" {
				continue
			}
			host, port, err := net.SplitHostPort(l.Address)
			if err != nil {
				continue
			}
			if host == "0.0.0.0" {
				host = "127.0.0.1"
			}
			ip, e := netip.ParseAddr(host)
			if e != nil || !ip.Is4() {
				continue
			}
			// Only daemon-owned, configured local listener addresses are probed.
			transport := &http.Transport{Proxy: nil, DialContext: (&net.Dialer{Timeout: time.Second}).DialContext, ResponseHeaderTimeout: time.Second, MaxResponseHeaderBytes: 16 << 10, DisableKeepAlives: true}
			client := &http.Client{Transport: transport, Timeout: time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
			request, _ := http.NewRequestWithContext(r.Context(), http.MethodGet, "http://"+net.JoinHostPort(host, port)+"/health", nil)
			response, e := client.Do(request)
			result := map[string]any{"address": l.Address, "http_responded": e == nil, "http_playback_verified": false, "port_availability": "owned by this daemon"}
			if response != nil {
				result["http_status"] = response.StatusCode
				_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
				response.Body.Close()
			}
			transport.CloseIdleConnections()
			if e != nil {
				result["error"] = e.Error()
			}
			results = append(results, result)
		}
		writeJSON(w, 200, map[string]any{"results": results, "note": "An HTTP response (including denial or unknown route) verifies listener connectivity, not TS validity or successful playback"})
		return
	}
	if action != "interface" && action != "source" && action != "multicast_eligibility" && action != "route" {
		streamError(w, errors.New("unsupported safe diagnostic action"))
		return
	}
	if s.options.Streams == nil {
		unavailable(w)
		return
	}
	v, err := s.options.Streams.Get(id)
	if err != nil {
		streamError(w, err)
		return
	}
	valid := v.Channel.ValidateWithSources(s.options.Streams.Sources())
	selection := interfaceError(v.Channel)
	writeJSON(w, 200, map[string]any{"channel_id": id, "configuration_valid": valid == nil, "interface_error": selection, "source_error": errorText(valid), "selected_interface": v.Channel.Interface, "route_selection": "explicit local interface only; no arbitrary destinations probed", "multicast_eligible": valid == nil && selection == nil && v.Channel.SourceType == "multicast", "socket_bound": v.SocketBound, "membership_joined": v.MembershipJoined, "last_datagram_at": v.LastDatagramAt, "last_valid_ts_at": v.LastPacketAt, "http_payload_bytes_delivered": v.Counters.OutputBytes})
}
func errorText(err error) any {
	if err != nil {
		return err.Error()
	}
	return nil
}
