// Package config owns startup configuration. It has no dependency on the UI or stream engine.
package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/udphttp-proxy/udphttp-proxy/internal/stream"

	"go.yaml.in/yaml/v3"
)

type Duration time.Duration

func (d *Duration) UnmarshalYAML(n *yaml.Node) error {
	if n.Tag != "!!str" {
		return errors.New("duration must be a string, for example 10s")
	}
	v, err := time.ParseDuration(n.Value)
	if err != nil {
		return err
	}
	*d = Duration(v)
	return nil
}
func (d Duration) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("%q", time.Duration(d).String())), nil
}
func (d Duration) Value() time.Duration { return time.Duration(d) }
func (d *Duration) UnmarshalJSON(data []byte) error {
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return errors.New("duration must be a string")
	}
	parsed, err := time.ParseDuration(value)
	if err == nil {
		*d = Duration(parsed)
	}
	return err
}

type Config struct {
	Management Management       `yaml:"management" json:"management"`
	Network    Network          `yaml:"network" json:"network"`
	Streaming  Streaming        `yaml:"streaming" json:"streaming"`
	Storage    Storage          `yaml:"storage" json:"storage"`
	Logging    Logging          `yaml:"logging" json:"logging"`
	Channels   []stream.Channel `yaml:"channels" json:"channels"`
	Dashboard  Dashboard        `yaml:"dashboard" json:"dashboard"`
}
type Dashboard struct {
	Name               string `yaml:"name" json:"name"`
	Theme              string `yaml:"theme" json:"theme"`
	RefreshSeconds     int    `yaml:"refresh_seconds" json:"refresh_seconds"`
	DefaultStartup     string `yaml:"default_startup" json:"default_startup"`
	DefaultIdleSeconds int    `yaml:"default_idle_seconds" json:"default_idle_seconds"`
}
type Management struct {
	Host            string   `yaml:"host" json:"host"`
	Port            int      `yaml:"port" json:"port"`
	ReadTimeout     Duration `yaml:"read_timeout" json:"read_timeout"`
	WriteTimeout    Duration `yaml:"write_timeout" json:"write_timeout"`
	IdleTimeout     Duration `yaml:"idle_timeout" json:"idle_timeout"`
	ShutdownTimeout Duration `yaml:"shutdown_timeout" json:"shutdown_timeout"`
}
type Network struct {
	MulticastInterface string   `yaml:"multicast_interface" json:"multicast_interface"`
	HTTPInterface      string   `yaml:"http_interface" json:"http_interface"`
	HTTPPort           int      `yaml:"http_port" json:"http_port"`
	SecondaryHTTPPort  int      `yaml:"secondary_http_port" json:"secondary_http_port"`
	HTTPBindAddresses  []string `yaml:"http_bind_addresses" json:"http_bind_addresses"`
}
type Streaming struct {
	Enabled                  bool     `yaml:"enabled" json:"enabled"`
	UDPReceiveBuffer         int      `yaml:"udp_receive_buffer" json:"udp_receive_buffer"`
	StreamBuffer             int      `yaml:"stream_buffer" json:"stream_buffer"`
	MaxClients               int      `yaml:"max_clients" json:"max_clients"`
	ConnectTimeout           Duration `yaml:"connect_timeout" json:"connect_timeout"`
	AllowedClientNetworks    []string `yaml:"allowed_client_networks" json:"allowed_client_networks"`
	AllowedMulticastGroups   []string `yaml:"allowed_multicast_groups" json:"allowed_multicast_groups"`
	AllowedUnicastSources    []string `yaml:"allowed_unicast_sources" json:"allowed_unicast_sources"`
	BlockedMulticastNetworks []string `yaml:"blocked_multicast_networks" json:"blocked_multicast_networks"`
	AllowLoopbackUnicast     bool     `yaml:"allow_loopback_unicast" json:"allow_loopback_unicast"`
	MaxStreams               int      `yaml:"max_streams" json:"max_streams"`
	ClientBufferBytes        int      `yaml:"client_buffer_bytes" json:"client_buffer_bytes"`
	MaxBufferedBytes         int      `yaml:"max_buffered_bytes" json:"max_buffered_bytes"`
	InactivityTimeout        Duration `yaml:"inactivity_timeout" json:"inactivity_timeout"`
	ClientWriteTimeout       Duration `yaml:"client_write_timeout" json:"client_write_timeout"`
}
type Storage struct {
	Path string `yaml:"path" json:"path"`
}
type Logging struct {
	Level          string `yaml:"level" json:"level"`
	RetentionCount int    `yaml:"retention_count" json:"retention_count"`
	RetentionDays  int    `yaml:"retention_days" json:"retention_days"`
}

func Default() Config {
	return Config{
		Management: Management{"127.0.0.1", 8080, Duration(10 * time.Second), Duration(15 * time.Second), Duration(60 * time.Second), Duration(10 * time.Second)},
		Network:    Network{HTTPInterface: "127.0.0.1", HTTPPort: 7781, SecondaryHTTPPort: 7782},
		Streaming:  Streaming{Enabled: true, UDPReceiveBuffer: 4 << 20, StreamBuffer: 8 << 20, MaxClients: 100, ConnectTimeout: Duration(10 * time.Second), AllowedClientNetworks: []string{}, AllowedMulticastGroups: []string{}, AllowedUnicastSources: []string{}, BlockedMulticastNetworks: []string{"224.0.0.0/24"}, MaxStreams: 128, ClientBufferBytes: 256 << 10, MaxBufferedBytes: 64 << 20, InactivityTimeout: Duration(10 * time.Second), ClientWriteTimeout: Duration(10 * time.Second)},
		Storage:    Storage{"data/udphttp-proxy.db"}, Logging: Logging{Level: "info", RetentionCount: 2000, RetentionDays: 7},
		Dashboard: Dashboard{Name: "UDPHTTP Proxy", Theme: "system", RefreshSeconds: 5, DefaultStartup: "on_demand", DefaultIdleSeconds: 5},
		Channels:  []stream.Channel{},
	}
}

// Load applies strict YAML over defaults. Only documented environment overrides are accepted.
func Load(path string, getenv func(string) string) (Config, error) {
	cfg := Default()
	if path != "" {
		f, err := os.Open(path)
		if err != nil {
			return cfg, fmt.Errorf("open config: %w", err)
		}
		defer f.Close()
		content, readErr := io.ReadAll(io.LimitReader(f, (1<<20)+1))
		if readErr != nil {
			return cfg, fmt.Errorf("read config: %w", readErr)
		}
		if len(content) > 1<<20 {
			return cfg, errors.New("config exceeds 1 MiB limit")
		}
		dec := yaml.NewDecoder(bytes.NewReader(content))
		dec.KnownFields(true)
		if err = dec.Decode(&cfg); err != nil {
			return cfg, fmt.Errorf("decode config: %w", err)
		}
		var extra any
		if err = dec.Decode(&extra); err != io.EOF {
			return cfg, errors.New("config must contain exactly one YAML document")
		}
	}
	if getenv != nil {
		if v := getenv("UDPHTTP_STORAGE_PATH"); v != "" {
			cfg.Storage.Path = v
		}
		if v := getenv("UDPHTTP_LOG_LEVEL"); v != "" {
			cfg.Logging.Level = v
		}
	}
	return cfg, cfg.Validate()
}

var interfaceName = regexp.MustCompile(`^[a-zA-Z0-9_][a-zA-Z0-9_.:-]{0,14}$`)
var numericAddress = regexp.MustCompile(`^[0-9.]+$`)

func validInterface(s string) bool {
	if s == "" {
		return true
	}
	if ip, err := netip.ParseAddr(s); err == nil {
		return ip.Is4() && !ip.IsMulticast() && !ip.IsUnspecified() && ip.String() != "255.255.255.255"
	}
	if strings.Contains(s, ".") && numericAddress.MatchString(s) {
		return false
	}
	return interfaceName.MatchString(s) && !strings.Contains(s, "..") && !strings.Contains(s, ":")
}
func validPort(p int) bool { return p > 0 && p <= 65535 }

func (c Config) Validate() error {
	var issues []error
	add := func(ok bool, msg string) {
		if !ok {
			issues = append(issues, errors.New(msg))
		}
	}
	ip, err := netip.ParseAddr(c.Management.Host)
	add(err == nil && ip.Is4() && ip.IsLoopback(), "management.host must be a literal IPv4 loopback address; authenticated remote administration is unavailable")
	add(validPort(c.Management.Port), "management.port must be 1..65535")
	for name, d := range map[string]Duration{"read_timeout": c.Management.ReadTimeout, "write_timeout": c.Management.WriteTimeout, "idle_timeout": c.Management.IdleTimeout, "shutdown_timeout": c.Management.ShutdownTimeout, "connect_timeout": c.Streaming.ConnectTimeout, "inactivity_timeout": c.Streaming.InactivityTimeout, "client_write_timeout": c.Streaming.ClientWriteTimeout} {
		add(d > 0 && d <= Duration(24*time.Hour), name+" must be greater than zero and at most 24h")
	}
	add(validInterface(c.Network.MulticastInterface), "network.multicast_interface must be an interface name or unicast IPv4 address")
	add(validInterface(c.Network.HTTPInterface), "network.http_interface must be an interface name or unicast IPv4 address")
	add(len(c.Network.HTTPBindAddresses) <= 8, "at most eight http_bind_addresses are supported")
	seenBinds := map[string]bool{}
	for _, address := range c.Network.HTTPBindAddresses {
		ip, e := netip.ParseAddr(address)
		add(e == nil && ip.Is4() && !ip.IsMulticast() && address != "255.255.255.255", "http_bind_addresses must be literal local IPv4 addresses or 0.0.0.0")
		add(!seenBinds[address], "duplicate HTTP bind address")
		seenBinds[address] = true
	}
	add(!seenBinds["0.0.0.0"] || len(seenBinds) == 1, "wildcard HTTP bind cannot be combined with specific addresses")
	add(validPort(c.Network.HTTPPort), "network.http_port must be 1..65535")
	add(c.Network.SecondaryHTTPPort == 0 || validPort(c.Network.SecondaryHTTPPort), "network.secondary_http_port must be 0 (disabled) or 1..65535")
	add(c.Network.SecondaryHTTPPort == 0 || c.Network.SecondaryHTTPPort != c.Network.HTTPPort, "streaming HTTP ports must differ")
	add(c.Management.Port != c.Network.HTTPPort && c.Management.Port != c.Network.SecondaryHTTPPort, "management and streaming HTTP ports must differ")
	add(c.Streaming.UDPReceiveBuffer >= 64<<10 && c.Streaming.UDPReceiveBuffer <= 64<<20, "streaming.udp_receive_buffer must be 64 KiB..64 MiB")
	add(c.Streaming.StreamBuffer >= 64<<10 && c.Streaming.StreamBuffer <= 256<<20, "streaming.stream_buffer must be 64 KiB..256 MiB")
	add(c.Streaming.MaxClients >= 1 && c.Streaming.MaxClients <= 100000, "streaming.max_clients must be 1..100000")
	add(c.Streaming.MaxStreams >= 1 && c.Streaming.MaxStreams <= 1024, "streaming.max_streams must be 1..1024")
	add(c.Streaming.ClientBufferBytes >= 564 && c.Streaming.ClientBufferBytes <= 8<<20 && c.Streaming.ClientBufferBytes <= c.Streaming.StreamBuffer, "streaming.client_buffer_bytes must be 564 bytes..8 MiB and within stream_buffer")
	add(c.Streaming.MaxBufferedBytes >= 64<<10 && c.Streaming.MaxBufferedBytes <= 1<<30 && c.Streaming.MaxBufferedBytes >= c.Streaming.ClientBufferBytes, "max_buffered_bytes must be 64 KiB..1 GiB and cover a client buffer")
	for _, s := range c.Streaming.AllowedClientNetworks {
		p, e := netip.ParsePrefix(s)
		add(e == nil && p.Addr().Is4() && p == p.Masked(), "allowed_client_networks requires canonical IPv4 CIDRs")
	}
	for _, s := range c.Streaming.AllowedMulticastGroups {
		p, e := netip.ParseAddr(s)
		add(e == nil && p.Is4() && p.IsMulticast(), "allowed_multicast_groups requires literal IPv4 multicast addresses")
	}
	for _, s := range c.Streaming.AllowedUnicastSources {
		p, e := netip.ParseAddr(s)
		add(e == nil && p.Is4() && (p.IsGlobalUnicast() || (p.IsLoopback() && c.Streaming.AllowLoopbackUnicast)), "allowed_unicast_sources requires literal unicast IPv4 addresses; loopback needs explicit permission")
	}
	for _, s := range c.Streaming.BlockedMulticastNetworks {
		p, e := netip.ParsePrefix(s)
		add(e == nil && p.Addr().Is4() && p.Bits() >= 4 && p == p.Masked() && p.Addr().IsMulticast(), "blocked_multicast_networks requires canonical IPv4 multicast CIDRs")
	}
	add(len(c.Channels) <= c.Streaming.MaxStreams, "channels exceeds max_streams")
	ids := map[string]bool{}
	for _, channel := range c.Channels {
		v := channel.Defaults(c.Streaming.MaxClients, c.Streaming.ClientBufferBytes, c.Streaming.UDPReceiveBuffer)
		if e := v.Validate(c.StreamPolicy()); e != nil {
			issues = append(issues, fmt.Errorf("channel %q: %w", v.ID, e))
		}
		add(!ids[v.ID], "duplicate channel ID")
		ids[v.ID] = true
		add(v.MaxClients <= c.Streaming.MaxClients && v.ClientBufferBytes <= c.Streaming.StreamBuffer && v.ClientBufferBytes <= c.Streaming.MaxBufferedBytes, "channel exceeds global client/buffer limits")
	}
	add(strings.TrimSpace(c.Storage.Path) != "" && !strings.ContainsAny(c.Storage.Path, "\x00\r\n") && c.Storage.Path != ":memory:" && !strings.HasPrefix(c.Storage.Path, "file:"), "storage.path must be a filesystem path, not a SQLite URI")
	add(c.Logging.Level == "debug" || c.Logging.Level == "info" || c.Logging.Level == "warn" || c.Logging.Level == "error", "logging.level must be debug, info, warn, or error")
	add(c.Logging.RetentionCount >= 100 && c.Logging.RetentionCount <= 10000, "logging.retention_count must be 100..10000")
	add(c.Logging.RetentionDays >= 1 && c.Logging.RetentionDays <= 30, "logging.retention_days must be 1..30")
	add(len(strings.TrimSpace(c.Dashboard.Name)) > 0 && len(c.Dashboard.Name) <= 64 && !strings.ContainsAny(c.Dashboard.Name, "\r\n\x00"), "dashboard.name must contain 1..64 printable bytes")
	add(c.Dashboard.Theme == "light" || c.Dashboard.Theme == "dark" || c.Dashboard.Theme == "system", "dashboard.theme must be light, dark, or system")
	add(c.Dashboard.RefreshSeconds >= 3 && c.Dashboard.RefreshSeconds <= 60, "dashboard.refresh_seconds must be 3..60")
	add(c.Dashboard.DefaultStartup == "always" || c.Dashboard.DefaultStartup == "on_demand", "dashboard.default_startup is invalid")
	add(c.Dashboard.DefaultIdleSeconds >= 0 && c.Dashboard.DefaultIdleSeconds <= 86400, "dashboard.default_idle_seconds must be 0..86400")
	return errors.Join(issues...)
}

func (c Config) StreamPolicy() stream.Policy {
	return stream.Policy{AllowedMulticastGroups: c.Streaming.AllowedMulticastGroups, BlockedMulticastNetworks: c.Streaming.BlockedMulticastNetworks, AllowedUnicastSources: c.Streaming.AllowedUnicastSources, AllowLoopbackUnicast: c.Streaming.AllowLoopbackUnicast}
}
