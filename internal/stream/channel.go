// Package stream implements transport and fan-out independently of the HTTP API and UI.
package stream

import (
	"errors"
	"fmt"
	"net/netip"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/udphttp-proxy/udphttp-proxy/internal/network"
)

type Channel struct {
	LogoURL               string `json:"logo_url" yaml:"logo_url"`
	TVGID                 string `json:"tvg_id" yaml:"tvg_id"`
	TVGName               string `json:"tvg_name" yaml:"tvg_name"`
	Group                 string `json:"group" yaml:"group"`
	SortOrder             int    `json:"sort_order" yaml:"sort_order"`
	Description           string `json:"description" yaml:"description"`
	PreferredPlaybackPort string `json:"preferred_playback_port" yaml:"preferred_playback_port"`
	ID                    string `json:"id" yaml:"id"`
	Name                  string `json:"name" yaml:"name"`
	SourceType            string `json:"source_type" yaml:"source_type"`
	SourceAddress         string `json:"source_address" yaml:"source_address"`
	SourcePort            int    `json:"source_port" yaml:"source_port"`
	Interface             string `json:"interface" yaml:"interface"`
	Enabled               bool   `json:"enabled" yaml:"enabled"`
	IdleTimeoutSeconds    int    `json:"idle_timeout_seconds" yaml:"idle_timeout_seconds"`
	MaxClients            int    `json:"max_clients" yaml:"max_clients"`
	ClientBufferBytes     int    `json:"client_buffer_bytes" yaml:"client_buffer_bytes"`
	UDPReceiveBuffer      int    `json:"udp_receive_buffer" yaml:"udp_receive_buffer"`
	Startup               string `json:"startup" yaml:"startup"`
}
type Policy struct {
	AllowedMulticastGroups   []string
	BlockedMulticastNetworks []string
	AllowedUnicastSources    []string
	AllowLoopbackUnicast     bool
}

var channelID = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$`)

func (c Channel) Defaults(maxClients, buffer, udpBuffer int) Channel {
	if c.PreferredPlaybackPort == "" {
		c.PreferredPlaybackPort = "primary"
	}
	if c.Startup == "" {
		c.Startup = "on_demand"
	}
	if c.MaxClients == 0 {
		c.MaxClients = maxClients
	}
	if c.ClientBufferBytes == 0 {
		c.ClientBufferBytes = buffer
	}
	if c.UDPReceiveBuffer == 0 {
		c.UDPReceiveBuffer = udpBuffer
	}
	return c
}
func (c Channel) Validate(p Policy) error {
	for _, value := range []string{c.TVGID, c.TVGName, c.Group} {
		if len(value) > 256 || strings.ContainsAny(value, "\"\r\n") {
			return errors.New("playlist metadata must be at most 256 bytes without quotes or line breaks")
		}
		for _, r := range value {
			if r < 32 || r == 127 {
				return errors.New("playlist metadata contains control characters")
			}
		}
	}
	if len(c.Description) > 2048 || c.SortOrder < 0 || c.SortOrder > 1000000 {
		return errors.New("invalid description length or sort_order (0..1000000)")
	}
	if c.PreferredPlaybackPort != "" && c.PreferredPlaybackPort != "primary" && c.PreferredPlaybackPort != "secondary" {
		return errors.New("preferred_playback_port must be primary or secondary")
	}
	if c.LogoURL != "" {
		u, err := url.Parse(c.LogoURL)
		if err != nil || len(c.LogoURL) > 2048 || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || strings.ContainsAny(c.LogoURL, "\"\r\n\t") {
			return errors.New("logo_url must be a bounded HTTP(S) URL without credentials or control characters")
		}
	}
	if !channelID.MatchString(c.ID) {
		return errors.New("id must contain 1..64 letters, digits, hyphens or underscores")
	}
	if len(c.Name) == 0 || len(c.Name) > 128 {
		return errors.New("name must contain 1..128 bytes")
	}
	for _, r := range c.Name {
		if r < 32 || r == 127 {
			return errors.New("name must not contain control characters")
		}
	}
	ip, err := netip.ParseAddr(c.SourceAddress)
	if err != nil || !ip.Is4() {
		return errors.New("source_address must be a literal IPv4 address")
	}
	if c.SourcePort < 1 || c.SourcePort > 65535 {
		return errors.New("source_port must be 1..65535")
	}
	if c.Interface == "" {
		return errors.New("an explicit interface name or assigned IPv4 address is required")
	}
	if local, e := netip.ParseAddr(c.Interface); e == nil {
		if !local.Is4() || local.IsMulticast() || local.IsUnspecified() {
			return errors.New("interface must be a local unicast IPv4 address")
		}
	} else if len(c.Interface) > 256 || strings.TrimSpace(c.Interface) == "" || strings.ContainsAny(c.Interface, "/\\\r\n\x00") {
		return errors.New("invalid interface name")
	}
	allowed := false
	switch c.SourceType {
	case "multicast":
		if !ip.IsMulticast() {
			return errors.New("multicast source_address must be in 224.0.0.0/4")
		}
		if netip.MustParsePrefix("224.0.0.0/24").Contains(ip) {
			return errors.New("reserved local-control multicast groups 224.0.0.0/24 are always blocked")
		}
		for _, cidr := range p.BlockedMulticastNetworks {
			prefix, e := netip.ParsePrefix(cidr)
			if e != nil {
				return errors.New("invalid blocked multicast policy")
			}
			if prefix.Contains(ip) {
				return fmt.Errorf("multicast group is blocked by %s", cidr)
			}
		}
		for _, address := range p.AllowedMulticastGroups {
			allowed = allowed || address == ip.String()
		}
	case "unicast":
		if !ip.IsGlobalUnicast() && !(ip.IsLoopback() && p.AllowLoopbackUnicast) {
			return errors.New("unicast source must be a unicast IPv4 sender; loopback requires explicit permission")
		}
		for _, address := range p.AllowedUnicastSources {
			allowed = allowed || address == ip.String()
		}
	default:
		return errors.New("source_type must be multicast or unicast")
	}
	if !allowed {
		return errors.New("source address is not in the corresponding server allowlist")
	}
	if c.IdleTimeoutSeconds < 0 || c.IdleTimeoutSeconds > 86400 {
		return errors.New("idle_timeout_seconds must be 0..86400")
	}
	if c.MaxClients < 1 || c.MaxClients > 100000 {
		return errors.New("max_clients must be 1..100000")
	}
	if c.ClientBufferBytes < 188*3 || c.ClientBufferBytes > 8<<20 {
		return errors.New("client_buffer_bytes must be 564 bytes..8 MiB")
	}
	if c.UDPReceiveBuffer < 64<<10 || c.UDPReceiveBuffer > 64<<20 {
		return errors.New("udp_receive_buffer must be 64 KiB..64 MiB")
	}
	if c.Startup != "on_demand" && c.Startup != "always" {
		return errors.New("startup must be on_demand or always")
	}
	return nil
}

// SourceKey normalizes interface names and equivalent assigned addresses when available.
// Duplicate channel sources are rejected rather than creating duplicate upstream receivers.
func (c Channel) SourceKey() string {
	iface, bind, err := resolveInterface(c.Interface, c.SourceType == "multicast")
	selection := c.Interface
	if err == nil {
		selection = iface.Name
		if c.SourceType == "unicast" {
			selection += "@" + bind.String()
		}
	}
	return fmt.Sprintf("%s/%s:%d/%s", c.SourceType, c.SourceAddress, c.SourcePort, selection)
}
func resolveInterface(value string, multicast bool) (*network.Interface, netip.Addr, error) {
	items, err := network.Discover()
	if err != nil {
		return nil, netip.Addr{}, err
	}
	item, err := network.Resolve(value, items, multicast)
	if err != nil {
		return nil, netip.Addr{}, err
	}
	if item == nil {
		return nil, netip.Addr{}, errors.New("interface is required")
	}
	ip, err := netip.ParseAddr(value)
	if err != nil {
		ip, err = netip.ParseAddr(item.IPv4[0])
	}
	return item, ip, err
}

type RecentError struct {
	Time      time.Time `json:"time"`
	Operation string    `json:"operation"`
	Message   string    `json:"message"`
}
type Counters struct {
	SyncErrors            uint64 `json:"sync_errors"`
	ReceiverRestarts      uint64 `json:"receiver_restarts"`
	Recoveries            uint64 `json:"recoveries"`
	RecoveryFailures      uint64 `json:"recovery_failures"`
	InputBytes            uint64 `json:"input_bytes"`
	OutputBytes           uint64 `json:"output_bytes"`
	Datagrams             uint64 `json:"datagrams"`
	Packets               uint64 `json:"packets"`
	CorruptPackets        uint64 `json:"corrupt_packets"`
	DiscardedBytes        uint64 `json:"discarded_bytes"`
	ContinuityErrors      uint64 `json:"continuity_errors"`
	Discontinuities       uint64 `json:"discontinuities"`
	DuplicatePackets      uint64 `json:"duplicate_packets"`
	SlowClientDisconnects uint64 `json:"slow_client_disconnects"`
	Retries               uint64 `json:"retries"`
	RejectedDatagrams     uint64 `json:"rejected_datagrams"`
}
type Snapshot struct {
	Health           string        `json:"health"`
	LastDatagramAt   *time.Time    `json:"last_datagram_at"`
	LastRecoveryAt   *time.Time    `json:"last_recovery_at"`
	SocketBound      bool          `json:"socket_bound"`
	MembershipJoined bool          `json:"membership_joined"`
	StartedAt        *time.Time    `json:"started_at"`
	Channel          Channel       `json:"channel"`
	State            string        `json:"state"`
	Clients          int           `json:"clients"`
	UptimeSeconds    int64         `json:"uptime_seconds"`
	LastPacketAt     *time.Time    `json:"last_packet_at"`
	InputBitrate     float64       `json:"input_bitrate"`
	OutputBitrate    float64       `json:"output_bitrate"`
	Counters         Counters      `json:"counters"`
	Errors           []RecentError `json:"errors"`
}
type ClientSnapshot struct {
	ID              uint64    `json:"id"`
	ChannelID       string    `json:"channel_id"`
	RemoteIP        string    `json:"remote_ip"`
	ConnectedAt     time.Time `json:"connected_at"`
	QueuedBytes     int       `json:"queued_bytes"`
	Closing         bool      `json:"closing"`
	DurationSeconds int64     `json:"duration_seconds"`
	BytesDelivered  uint64    `json:"bytes_delivered"`
	OutputBitrate   float64   `json:"output_bitrate"`
}
