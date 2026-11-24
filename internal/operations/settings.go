package operations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"reflect"
	"strconv"
	"sync"

	"github.com/udphttp-proxy/udphttp-proxy/internal/config"
	"github.com/udphttp-proxy/udphttp-proxy/internal/netpolicy"
	"github.com/udphttp-proxy/udphttp-proxy/internal/network"
	"github.com/udphttp-proxy/udphttp-proxy/internal/stream"
)

var ErrRevision = errors.New("settings revision changed; reload before saving")
var ErrPersistence = errors.New("settings storage failed")

type Timeouts struct {
	Read     config.Duration `json:"read_timeout"`
	Write    config.Duration `json:"write_timeout"`
	Idle     config.Duration `json:"idle_timeout"`
	Shutdown config.Duration `json:"shutdown_timeout"`
}
type Settings struct {
	Dashboard config.Dashboard `json:"dashboard"`
	Network   config.Network   `json:"network"`
	Streaming config.Streaming `json:"streaming"`
	Logging   config.Logging   `json:"logging"`
	Timeouts  Timeouts         `json:"timeouts"`
}
type Record struct {
	Revision         int64    `json:"revision"`
	AcceptedRevision int64    `json:"accepted_revision"`
	Desired          Settings `json:"settings"`
	Accepted         Settings `json:"accepted"`
	LastError        string   `json:"last_error"`
}
type SettingsRepository interface {
	InitServerSettings(context.Context, Settings) error
	ReadServerSettings(context.Context) (Record, error)
	WriteServerSettings(context.Context, int64, Settings) (Record, error)
	AcceptServerSettings(context.Context, int64) error
	RollbackServerSettings(context.Context, string) (Record, error)
	ListChannels(context.Context) ([]stream.Channel, error)
}
type SettingsManager struct {
	mu           sync.Mutex
	base, active config.Config
	repo         SettingsRepository
	listeners    []string
	activePolicy config.NetworkPolicies
	viewRevision int64
	viewSettings Settings
	Live         func(config.Dashboard, config.Logging)
	LiveAccess   func(netpolicy.Sources, netpolicy.Prefixes)
}

func FromConfig(c config.Config) Settings {
	return Settings{c.Dashboard, c.Network, c.Streaming, c.Logging, Timeouts{c.Management.ReadTimeout, c.Management.WriteTimeout, c.Management.IdleTimeout, c.Management.ShutdownTimeout}}
}
func (s Settings) Apply(base config.Config) config.Config {
	base.Dashboard = s.Dashboard
	base.Network = s.Network
	base.Streaming = s.Streaming
	base.Logging = s.Logging
	base.Management.ReadTimeout = s.Timeouts.Read
	base.Management.WriteTimeout = s.Timeouts.Write
	base.Management.IdleTimeout = s.Timeouts.Idle
	base.Management.ShutdownTimeout = s.Timeouts.Shutdown
	return base
}
func NewSettings(ctx context.Context, base config.Config, repo SettingsRepository) (*SettingsManager, error) {
	policies, err := base.NormalizeNetwork()
	if err != nil {
		return nil, err
	}
	if err := repo.InitServerSettings(ctx, FromConfig(base)); err != nil {
		return nil, err
	}
	return &SettingsManager{base: base, active: base, activePolicy: policies, repo: repo, listeners: []string{}, viewRevision: -1}, nil
}
func (m *SettingsManager) Startup(ctx context.Context, ignore bool) (config.Config, error) {
	r, err := m.repo.ReadServerSettings(ctx)
	if err != nil {
		return m.base, err
	}
	if ignore {
		return m.base, nil
	}
	candidate := r.Desired.Apply(m.base)
	candidate.Channels = nil
	policies, err := candidate.NormalizeNetwork()
	if err != nil || candidate.ValidateNormalized(policies) != nil {
		_, _ = m.repo.RollbackServerSettings(ctx, "Stored settings failed validation; restored accepted revision")
		return m.restoreConfig(r.Accepted)
	}
	channels, e := m.repo.ListChannels(ctx)
	if e != nil {
		return m.base, e
	}
	if len(channels) > candidate.Streaming.MaxStreams {
		_, _ = m.repo.RollbackServerSettings(ctx, "Staged max_streams is below the current channel count; restored accepted settings")
		return m.restoreConfig(r.Accepted)
	}
	candidate.Channels = m.base.Channels
	return candidate, nil
}
func (m *SettingsManager) Accepted(ctx context.Context) (config.Config, error) {
	r, err := m.repo.ReadServerSettings(ctx)
	if err != nil {
		return m.base, err
	}
	return m.restoreConfig(r.Accepted)
}
func (m *SettingsManager) restoreConfig(settings Settings) (config.Config, error) {
	c := settings.Apply(m.base)
	c.Channels = nil
	policies, err := c.NormalizeNetwork()
	if err != nil {
		return m.base, err
	}
	if err := c.ValidateNormalized(policies); err != nil {
		return m.base, err
	}
	c.Channels = m.base.Channels
	return c, nil
}
func (m *SettingsManager) Activate(ctx context.Context, c config.Config, accept bool) error {
	policies, err := c.NormalizeNetwork()
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.active, m.activePolicy = c, policies
	if !accept {
		return nil
	}
	r, err := m.repo.ReadServerSettings(ctx)
	if err != nil {
		return err
	}
	return m.repo.AcceptServerSettings(ctx, r.Revision)
}
func (m *SettingsManager) SetListeners(addresses []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.listeners = append([]string{}, addresses...)
}
func (m *SettingsManager) View(ctx context.Context) (map[string]any, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, err := m.repo.ReadServerSettings(ctx)
	if err != nil {
		return nil, err
	}
	if r.Revision != m.viewRevision {
		desired := r.Desired.Apply(m.base)
		if _, err := desired.NormalizeNetwork(); err != nil {
			return nil, err
		}
		m.viewRevision, m.viewSettings = r.Revision, FromConfig(desired)
	}
	r.Desired = m.viewSettings
	authentication := "trusted local user; remote binding prohibited"
	forwarded := "ignored"
	proxies := []string{}
	origins := []string{}
	if m.base.Management.ReverseProxy.Enabled {
		authentication = "trusted local users; reverse proxy enforces remote authentication and client authorization"
		forwarded = "single-hop X-Forwarded-Host/Proto/For and optional Port; configured TCP peers and exact origins only"
		proxies = append(proxies, m.base.Management.ReverseProxy.TrustedProxies...)
		origins = append(origins, m.base.Management.ReverseProxy.TrustedOrigins...)
	}
	return map[string]any{
		"settings": r.Desired, "active": FromConfig(m.active), "revision": r.Revision,
		"accepted_revision": r.AcceptedRevision, "restart_required": restartRequired(r.Desired, FromConfig(m.active)), "last_error": r.LastError,
		"read_only": map[string]any{
			"advertised_playback_host": m.base.Playlist.AdvertisedHost,
			"remote_playlist_access":   map[bool]string{true: "Enabled for approved playback and playlist client networks", false: "Disabled"}[m.base.Playlist.RemoteEnabled],
			"management_binding":       net.JoinHostPort(m.base.Management.Host, strconv.Itoa(m.base.Management.Port)),
			"storage_path":             m.base.Storage.Path, "authentication": authentication,
			"trusted_proxies": proxies, "playlist_allowed_client_networks": m.base.Playlist.AllowedClientNetworks, "trusted_origins": origins, "forwarded_headers": forwarded,
			"playback_authorization": "actual-peer CIDR allowlist; bearer tokens unavailable",
			"recovery_policy":        fmt.Sprintf("%s..%s exponential retry with %.0f%% jitter; rejoin after %s; YAML-only", m.base.Recovery.MinDelay.Value(), m.base.Recovery.MaxDelay.Value(), m.base.Recovery.Jitter*100, m.base.Recovery.RejoinAfter.Value()),
			"startup_service":        "systemd controls process startup; channel startup is individually persisted",
		},
	}, nil
}
func copyLiveAccess(dst *config.Streaming, src config.Streaming) {
	dst.AllowedClientNetworks = src.AllowedClientNetworks
	dst.AllowedMulticastGroups = src.AllowedMulticastGroups
	dst.AllowedUnicastSources = src.AllowedUnicastSources
	dst.BlockedMulticastNetworks = src.BlockedMulticastNetworks
	dst.AllowLoopbackUnicast = src.AllowLoopbackUnicast
}

func restartRequired(a, b Settings) bool {
	a.Dashboard = b.Dashboard
	a.Logging = b.Logging
	copyLiveAccess(&a.Streaming, b.Streaming)
	return !reflect.DeepEqual(a, b)
}
func (m *SettingsManager) Validate(ctx context.Context, s Settings) ([]string, error) {
	_, _, notes, err := m.prepare(ctx, s)
	return notes, err
}
func (m *SettingsManager) Preview(ctx context.Context, s Settings) (Settings, []string, bool, error) {
	normalized, policies, notes, err := m.prepare(ctx, s)
	m.mu.Lock()
	defer m.mu.Unlock()
	return normalized, notes, m.needsMulticastConfirmation(policies), err
}
func (m *SettingsManager) Policy(s *Settings) (config.NetworkPolicies, error) {
	if s == nil {
		m.mu.Lock()
		defer m.mu.Unlock()
		return m.activePolicy, nil
	}
	c := m.base
	c.Streaming = s.Streaming
	return c.NormalizeNetwork()
}
func (m *SettingsManager) prepare(ctx context.Context, s Settings) (Settings, config.NetworkPolicies, []string, error) {
	c := s.Apply(m.base)
	c.Channels = nil
	policies, err := c.NormalizeNetwork()
	if err == nil {
		err = c.ValidateNormalized(policies)
	}
	if err != nil {
		return s, policies, nil, err
	}
	s = FromConfig(c)
	notes := []string{"Playback access changes apply when saved. Streams and clients excluded by the new policy disconnect; saved channels remain editable."}
	if !restartRequired(s, FromConfig(m.Active())) {
		return s, policies, notes, nil
	}
	channels, err := m.repo.ListChannels(ctx)
	if err != nil {
		return s, policies, nil, fmt.Errorf("%w: %v", ErrPersistence, err)
	}
	if len(channels) > c.Streaming.MaxStreams {
		return s, policies, nil, errors.New("max_streams cannot be below the existing channel count")
	}
	for _, channel := range channels {
		v := channel.Defaults(c.Streaming.MaxClients, c.Streaming.ClientBufferBytes, c.Streaming.UDPReceiveBuffer)
		if v.MaxClients > c.Streaming.MaxClients || v.ClientBufferBytes > c.Streaming.StreamBuffer || v.ClientBufferBytes > c.Streaming.MaxBufferedBytes {
			return s, policies, nil, fmt.Errorf("channel %s exceeds proposed limits", v.ID)
		}
	}
	preflight, err := Preflight(c, m.listenerAddresses())
	return s, policies, append(notes, preflight...), err
}
func (m *SettingsManager) listenerAddresses() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string{}, m.listeners...)
}
func (m *SettingsManager) Save(ctx context.Context, revision int64, s Settings, confirmAllMulticast bool) (Record, error) {
	s, policies, _, err := m.prepare(ctx, s)
	if err != nil {
		return Record{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.needsMulticastConfirmation(policies) && !confirmAllMulticast {
		return Record{}, errors.New("confirm_all_multicast is required to allow all IPv4 multicast groups")
	}
	r, err := m.repo.WriteServerSettings(ctx, revision, s)
	if err != nil {
		if !errors.Is(err, ErrRevision) {
			err = fmt.Errorf("%w: %v", ErrPersistence, err)
		}
		return r, err
	}
	m.applyLive(s, policies)
	m.viewRevision, m.viewSettings = r.Revision, s
	return r, nil
}

func (m *SettingsManager) needsMulticastConfirmation(policies config.NetworkPolicies) bool {
	return policies.Sources.AllowsAllMulticast() && !m.activePolicy.Sources.AllowsAllMulticast()
}
func (m *SettingsManager) applyLive(s Settings, policies config.NetworkPolicies) {
	current := m.active.Streaming
	updated := current
	copyLiveAccess(&updated, s.Streaming)
	if m.LiveAccess != nil && !reflect.DeepEqual(updated, current) {
		m.LiveAccess(policies.Sources, policies.Playback)
	}
	m.active.Streaming = updated
	m.activePolicy = policies
	m.active.Dashboard = s.Dashboard
	m.active.Logging = s.Logging
	if m.Live != nil {
		m.Live(s.Dashboard, s.Logging)
	}
}
func (m *SettingsManager) Rollback(ctx context.Context, revision int64) (Record, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	current, err := m.repo.ReadServerSettings(ctx)
	if err != nil {
		return Record{}, err
	}
	if current.Revision != revision {
		return Record{}, ErrRevision
	}
	accepted := current.Accepted.Apply(m.base)
	accepted.Channels = nil
	policies, err := accepted.NormalizeNetwork()
	if err != nil {
		return Record{}, err
	}
	if err := accepted.ValidateNormalized(policies); err != nil {
		return Record{}, err
	}
	r, err := m.repo.RollbackServerSettings(ctx, "")
	if err == nil {
		m.applyLive(FromConfig(accepted), policies)
		m.viewRevision, m.viewSettings = r.Revision, FromConfig(accepted)
	}
	return r, err
}
func (m *SettingsManager) Reject(ctx context.Context, message string) error {
	_, err := m.repo.RollbackServerSettings(ctx, message)
	return err
}
func (m *SettingsManager) Active() config.Config { m.mu.Lock(); defer m.mu.Unlock(); return m.active }
func PlaybackAddresses(c config.Config) ([]string, error) {
	items, err := network.Discover()
	if err != nil {
		return nil, err
	}
	if _, err = network.Resolve(c.Network.MulticastInterface, items, true); err != nil {
		return nil, err
	}
	addresses := append([]string{}, c.Network.HTTPBindAddresses...)
	if len(addresses) == 0 {
		selected, e := network.Resolve(c.Network.HTTPInterface, items, false)
		if e != nil {
			return nil, e
		}
		if selected == nil {
			return nil, errors.New("select a playback interface")
		}
		if ip := net.ParseIP(c.Network.HTTPInterface); ip != nil {
			addresses = []string{ip.String()}
		} else {
			addresses = []string{selected.IPv4[0]}
		}
	}
	result := []string{}
	if !c.Streaming.Enabled {
		return result, nil
	}
	for _, address := range addresses {
		if address != "0.0.0.0" {
			if _, e := network.Resolve(address, items, false); e != nil {
				return nil, e
			}
		}
		for _, port := range []int{c.Network.HTTPPort, c.Network.SecondaryHTTPPort} {
			if port > 0 {
				result = append(result, net.JoinHostPort(address, strconv.Itoa(port)))
			}
		}
	}
	return result, nil
}
func Preflight(c config.Config, owned []string) ([]string, error) {
	addresses, err := PlaybackAddresses(c)
	if err != nil {
		return nil, err
	}
	notes := []string{"Availability is a point-in-time probe; active listeners change only after restart. Management binding is unchanged."}
	for _, address := range addresses {
		_, port, _ := net.SplitHostPort(address)
		skip := false
		for _, current := range owned {
			_, p, _ := net.SplitHostPort(current)
			if p == port {
				skip = true
				break
			}
		}
		if skip {
			notes = append(notes, address+": current service port; checked at restart")
			continue
		}
		l, e := net.Listen("tcp4", address)
		if e != nil {
			return nil, fmt.Errorf("port availability %s: %w", address, e)
		}
		_ = l.Close()
		notes = append(notes, address+": probe available")
	}
	return notes, nil
}
func Encode(s Settings) string { b, _ := json.Marshal(s); return string(b) }
