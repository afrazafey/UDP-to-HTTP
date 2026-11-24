package config

import "testing"

func TestProductionPolicyValidation(t *testing.T) {
	for name, change := range map[string]func(*Config){
		"reserved group": func(c *Config) {
			c.Streaming.AllowedMulticastGroups = []string{"224.0.0.1"}
			c.Streaming.BlockedMulticastNetworks = nil
		},
		"retry bounds":          func(c *Config) { c.Recovery.MaxDelay = 1 },
		"jitter":                func(c *Config) { c.Recovery.Jitter = 2 },
		"rejoin interval":       func(c *Config) { c.Recovery.RejoinAfter = 0 },
		"negative attempts":     func(c *Config) { c.Recovery.MaxAttempts = -1 },
		"advertised URL":        func(c *Config) { c.Playlist.AdvertisedHost = "http://server/path" },
		"advertised port":       func(c *Config) { c.Playlist.AdvertisedHost = "server:7781" },
		"remote without policy": func(c *Config) { c.Playlist.RemoteEnabled = true },
		"playlist CIDR":         func(c *Config) { c.Playlist.AllowedClientNetworks = []string{"192.168.10.2/33"} },
		"unknown log format":    func(c *Config) { c.LogFormat = "yaml" },
	} {
		t.Run(name, func(t *testing.T) {
			cfg := Default()
			change(&cfg)
			if err := cfg.Validate(); err == nil {
				t.Fatal("invalid production policy accepted")
			}
		})
	}
	valid := Default()
	valid.Playlist.AdvertisedHost = "playback.example"
	valid.Playlist.RemoteEnabled = true
	valid.Playlist.AllowedClientNetworks = []string{"192.168.10.0/24"}
	if err := valid.Validate(); err != nil {
		t.Fatal(err)
	}
}
