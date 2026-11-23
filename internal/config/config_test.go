package config

import (
	"github.com/udphttp-proxy/udphttp-proxy/internal/stream"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaults(t *testing.T) {
	if err := Default().Validate(); err != nil {
		t.Fatal(err)
	}
}
func TestLoadStrict(t *testing.T) {
	for _, test := range []struct {
		name, body string
		valid      bool
	}{
		{"partial", "management:\n  port: 8089\n", true},
		{"unknown", "management:\n  porrt: 8089\n", false},
		{"duplicate", "management:\n  port: 8089\n  port: 8090\n", false},
		{"remote", "management:\n  host: 0.0.0.0\n", false},
		{"hostname", "management:\n  host: localhost\n", false},
		{"duration", "management:\n  read_timeout: 10\n", false},
		{"negative duration", "management:\n  read_timeout: -10s\n", false},
		{"multiple docs", "management: {}\n---\nnetwork: {}\n", false},
		{"empty", "", false},
		{"bad YAML", "management: [", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte(test.body), 0600); err != nil {
				t.Fatal(err)
			}
			cfg, err := Load(path, nil)
			if (err == nil) != test.valid {
				t.Fatalf("valid=%v error=%v", test.valid, err)
			}
			if test.valid && cfg.Management.Port != 8089 {
				t.Fatal("override not loaded")
			}
		})
	}
}
func TestInvalidConfig(t *testing.T) {
	for name, change := range map[string]func(*Config){
		"management port":             func(c *Config) { c.Management.Port = 65536 },
		"stream port":                 func(c *Config) { c.Network.HTTPPort = 0 },
		"duplicate ports":             func(c *Config) { c.Network.SecondaryHTTPPort = c.Network.HTTPPort },
		"management stream collision": func(c *Config) { c.Management.Port = c.Network.HTTPPort },
		"bad interface":               func(c *Config) { c.Network.MulticastInterface = "../../eth0" },
		"multicast bind":              func(c *Config) { c.Network.HTTPInterface = "239.1.1.1" },
		"invalid numeric IP":          func(c *Config) { c.Network.HTTPInterface = "999.1.1.1" },
		"small buffer":                func(c *Config) { c.Streaming.UDPReceiveBuffer = 1 },
		"large buffer":                func(c *Config) { c.Streaming.StreamBuffer = 1 << 30 },
		"no clients":                  func(c *Config) { c.Streaming.MaxClients = 0 },
		"bad client CIDR":             func(c *Config) { c.Streaming.AllowedClientNetworks = []string{"192.168.1.1/24"} },
		"group hostname":              func(c *Config) { c.Streaming.AllowedMulticastGroups = []string{"example.com"} },
		"unicast group":               func(c *Config) { c.Streaming.AllowedUnicastSources = []string{"239.1.1.1"} },
		"unicast loopback":            func(c *Config) { c.Streaming.AllowedUnicastSources = []string{"127.0.0.1"} },
		"SQLite URI":                  func(c *Config) { c.Storage.Path = "file:test?mode=memory" },
		"empty path":                  func(c *Config) { c.Storage.Path = " " },
		"bad log level":               func(c *Config) { c.Logging.Level = "trace" },
	} {
		t.Run(name, func(t *testing.T) {
			c := Default()
			change(&c)
			if err := c.Validate(); err == nil {
				t.Fatal("invalid config accepted")
			}
		})
	}
}
func TestEnvironmentAndMissingFile(t *testing.T) {
	env := map[string]string{"UDPHTTP_STORAGE_PATH": "custom.db", "UDPHTTP_LOG_LEVEL": "debug"}
	c, err := Load("", func(k string) string { return env[k] })
	if err != nil {
		t.Fatal(err)
	}
	if c.Storage.Path != "custom.db" || c.Logging.Level != "debug" {
		t.Fatal("environment not applied")
	}
	if _, err = Load(filepath.Join(t.TempDir(), "missing.yaml"), nil); err == nil {
		t.Fatal("missing explicit config silently ignored")
	}
}
func TestValidStreamPolicy(t *testing.T) {
	c := Default()
	c.Network.MulticastInterface = "eth0"
	c.Network.SecondaryHTTPPort = 0
	c.Streaming.AllowedClientNetworks = []string{"192.168.0.0/16"}
	c.Streaming.AllowedMulticastGroups = []string{"239.1.1.1"}
	c.Streaming.AllowedUnicastSources = []string{"10.14.0.5"}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
}
func TestExample(t *testing.T) {
	if _, err := Load("../../configs/config.example.yaml", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := Load("../../deploy/config.yaml", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := Load("../../configs/phase2-local.yaml", nil); err != nil {
		t.Fatal(err)
	}
}

func TestPhase2Policy(t *testing.T) {
	c := Default()
	c.Streaming.AllowedMulticastGroups = []string{"224.0.0.10"}
	c.Channels = []stream.Channel{{ID: "reserved", Name: "Reserved", SourceType: "multicast", SourceAddress: "224.0.0.10", SourcePort: 1234, Interface: "eth0", Enabled: true}}
	if err := c.Validate(); err == nil {
		t.Fatal("reserved multicast accepted")
	}
	c.Streaming.BlockedMulticastNetworks = []string{}
	if err := c.Validate(); err != nil {
		t.Fatal("explicit reserved policy override", err)
	}
	c.Streaming.MaxBufferedBytes = 1
	if err := c.Validate(); err == nil {
		t.Fatal("invalid global budget accepted")
	}
}

func TestOversizeConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("#"+strings.Repeat("x", 1<<20)), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path, nil); err == nil {
		t.Fatal("oversize configuration accepted")
	}
}
