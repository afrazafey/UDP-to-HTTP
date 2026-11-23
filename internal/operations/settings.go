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
	"github.com/udphttp-proxy/udphttp-proxy/internal/network"
	"github.com/udphttp-proxy/udphttp-proxy/internal/stream"
)

var ErrRevision = errors.New("settings revision changed; reload before saving")

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
	Live         func(config.Dashboard, config.Logging)
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
	if err := repo.InitServerSettings(ctx, FromConfig(base)); err != nil {
		return nil, err
	}
	return &SettingsManager{base: base, active: base, repo: repo, listeners: []string{}}, nil
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
	if err = candidate.Validate(); err != nil {
		_, _ = m.repo.RollbackServerSettings(ctx, "Stored settings failed validation; restored accepted revision")
		return r.Accepted.Apply(m.base), nil
	}
	return candidate, nil
}
func (m *SettingsManager) Accepted(ctx context.Context) (config.Config, error) {
	r, err := m.repo.ReadServerSettings(ctx)
	return r.Accepted.Apply(m.base), err
}
func (m *SettingsManager) Activate(ctx context.Context, c config.Config, accept bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.active = c
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
	return map[string]any{"settings": r.Desired, "active": FromConfig(m.active), "revision": r.Revision, "accepted_revision": r.AcceptedRevision, "restart_required": restartRequired(r.Desired, FromConfig(m.active)), "last_error": r.LastError, "read_only": map[string]any{"management_binding": net.JoinHostPort(m.base.Management.Host, strconv.Itoa(m.base.Management.Port)), "storage_path": m.base.Storage.Path, "authentication": "trusted local user; remote binding prohibited", "trusted_proxies": []string{}, "forwarded_headers": "ignored", "playback_authorization": "actual-peer CIDR allowlist; bearer tokens unavailable", "recovery_policy": "250ms..30s exponential retry; read-only", "startup_service": "systemd controls process startup; channel startup is individually persisted"}}, nil
}
func restartRequired(a, b Settings) bool {
	a.Dashboard = b.Dashboard
	a.Logging = b.Logging
	return !reflect.DeepEqual(a, b)
}
func (m *SettingsManager) Validate(ctx context.Context, s Settings) ([]string, error) {
	c := s.Apply(m.base)
	c.Channels = nil
	if err := c.Validate(); err != nil {
		return nil, err
	}
	channels, err := m.repo.ListChannels(ctx)
	if err != nil {
		return nil, err
	}
	for _, channel := range channels {
		v := channel.Defaults(c.Streaming.MaxClients, c.Streaming.ClientBufferBytes, c.Streaming.UDPReceiveBuffer)
		if err = v.Validate(c.StreamPolicy()); err != nil {
			return nil, fmt.Errorf("channel %s: %w", v.ID, err)
		}
		if v.MaxClients > c.Streaming.MaxClients || v.ClientBufferBytes > c.Streaming.StreamBuffer || v.ClientBufferBytes > c.Streaming.MaxBufferedBytes {
			return nil, fmt.Errorf("channel %s exceeds proposed limits", v.ID)
		}
	}
	m.mu.Lock()
	owned := append([]string{}, m.listeners...)
	m.mu.Unlock()
	return Preflight(c, owned)
}
func (m *SettingsManager) Save(ctx context.Context, revision int64, s Settings) (Record, error) {
	if _, err := m.Validate(ctx, s); err != nil {
		return Record{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	r, err := m.repo.WriteServerSettings(ctx, revision, s)
	if err != nil {
		return r, err
	}
	m.active.Dashboard = s.Dashboard
	m.active.Logging = s.Logging
	if m.Live != nil {
		m.Live(s.Dashboard, s.Logging)
	}
	return r, nil
}
func (m *SettingsManager) Rollback(ctx context.Context) (Record, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, err := m.repo.RollbackServerSettings(ctx, "")
	if err == nil {
		m.active.Dashboard = r.Desired.Dashboard
		m.active.Logging = r.Desired.Logging
		if m.Live != nil {
			m.Live(r.Desired.Dashboard, r.Desired.Logging)
		}
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
