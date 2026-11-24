package config

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestNetworkListYAMLFormats(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	yaml := "network:\r\n  http_bind_addresses: \"10.14.0.23, 10.14.0.24\"\r\n" +
		"streaming:\r\n  allowed_client_networks:\r\n    - 10.14.0.5\r\n    - '192.168.1.81/24, 10.14.0.5/32, 2001:db8::1'\r\n" +
		"  allowed_multicast_groups: '239.0.1.20/8, 239.255.20.60/24'\r\n" +
		"  allowed_unicast_sources: |-\r\n    10.14.0.1,10.14.0.2\r\n    10.20.0.4/24\r\n" +
		"playlist:\r\n  allowed_client_networks: '10.14.0.5'\r\n" +
		"management:\r\n  reverse_proxy:\r\n    enabled: true\r\n    trusted_proxies: '127.0.0.1, 127.0.0.1'\r\n    trusted_origins: ['http://192.168.10.103']\r\n"
	if err := os.WriteFile(path, []byte(yaml), 0600); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct{ got, want []string }{
		{c.Streaming.AllowedClientNetworks, []string{"10.14.0.5", "192.168.1.0/24", "2001:db8::1"}},
		{c.Streaming.AllowedMulticastGroups, []string{"239.0.0.0/8", "239.255.20.0/24"}},
		{c.Streaming.AllowedUnicastSources, []string{"10.14.0.1", "10.14.0.2", "10.20.0.0/24"}},
		{c.Streaming.BlockedMulticastNetworks, []string{"224.0.0.0/24"}},
		{c.Playlist.AllowedClientNetworks, []string{"10.14.0.5"}},
		{c.Network.HTTPBindAddresses, []string{"10.14.0.23", "10.14.0.24"}},
		{c.Management.ReverseProxy.TrustedProxies, []string{"127.0.0.1"}},
	} {
		if !slices.Equal(test.got, test.want) {
			t.Errorf("want %v, got %v", test.want, test.got)
		}
	}
	c.Network.HTTPBindAddresses = []string{"0.0.0.0,10.14.0.23"}
	if c.Validate() == nil {
		t.Fatal("wildcard may not be mixed with an assigned address")
	}
}
