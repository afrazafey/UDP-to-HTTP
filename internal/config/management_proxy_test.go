package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestManagementOrigins(t *testing.T) {
	for _, origin := range []string{"http://[127.0.0.1]", "http://[example.com]", "http://2001:db8::1"} {
		if _, err := ParseManagementOrigin(origin); err == nil {
			t.Errorf("malformed authority accepted: %s", origin)
		}
	}
	for _, origin := range []string{"http://192.168.10.103", "https://dashboard.example", "https://dashboard.example:8443", "http://localhost:8080", "https://[2001:db8::1]:8443"} {
		if _, err := ParseManagementOrigin(origin); err != nil {
			t.Errorf("%q: %v", origin, err)
		}
	}
	for _, origin := range []string{"", "null", "*", "http://*.example", "//example.com", "ftp://example.com", "HTTP://example.com", "http://Example.com", "http://example.com/", "http://example.com/path", "http://example.com?", "http://example.com?q=1", "http://example.com#", "http://example.com#fragment", "http://user:pass@example.com", "http://example.com:", "http://example.com:0", "http://example.com:65536", "http://example.com:080", "http://example.com:abc", "http://999.1.1.1", "http://0.0.0.0", "http://239.1.1.1", "http://example.com.", "http://example.com,evil.example", "http://example.com http://evil.example", "http://example.com\\evil", "http://[fe80::1%25eth0]"} {
		t.Run(origin, func(t *testing.T) {
			if _, err := ParseManagementOrigin(origin); err == nil {
				t.Fatal("invalid origin accepted")
			}
		})
	}
}

func TestManagementProxyConfiguration(t *testing.T) {
	if Default().Management.ReverseProxy.Enabled {
		t.Fatal("proxy support must default to disabled")
	}
	valid := ManagementReverseProxy{Enabled: true, TrustedProxies: []string{"127.0.0.1"}, TrustedOrigins: []string{"http://192.168.10.103"}}
	for name, proxy := range map[string]ManagementReverseProxy{
		"valid":             valid,
		"no proxies":        {Enabled: true, TrustedOrigins: valid.TrustedOrigins},
		"no origins":        {Enabled: true, TrustedProxies: valid.TrustedProxies},
		"LAN peer":          {TrustedProxies: []string{"192.168.10.103"}},
		"CIDR":              {TrustedProxies: []string{"127.0.0.0/8"}},
		"hostname peer":     {TrustedProxies: []string{"localhost"}},
		"IPv6 peer":         {TrustedProxies: []string{"::1"}},
		"mapped peer":       {TrustedProxies: []string{"::ffff:127.0.0.1"}},
		"duplicate origin":  {TrustedOrigins: []string{"http://192.168.10.103", "http://192.168.10.103"}},
		"wildcard disabled": {TrustedOrigins: []string{"*"}},
	} {
		t.Run(name, func(t *testing.T) {
			cfg := Default()
			cfg.Management.ReverseProxy = proxy
			if err := cfg.Validate(); (err == nil) != (name == "valid") {
				t.Fatalf("unexpected validation result: %v", err)
			}
		})
	}
	cfg := Default()
	cfg.Management.ReverseProxy = valid
	cfg.Management.Host = "0.0.0.0"
	if cfg.Validate() == nil {
		t.Fatal("proxy support allowed remote management binding")
	}
	path := filepath.Join(t.TempDir(), "proxy.yaml")
	if err := os.WriteFile(path, []byte("management:\n  reverse_proxy:\n    enabled: true\n    trusted_proxies: [127.0.0.1]\n    trusted_origins: [http://192.168.10.103]\n"), 0600); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path, nil)
	if err != nil || !loaded.Management.ReverseProxy.Enabled || loaded.Management.ReverseProxy.TrustedOrigins[0] != valid.TrustedOrigins[0] {
		t.Fatalf("proxy YAML not loaded: %+v %v", loaded.Management.ReverseProxy, err)
	}
}
