package config

import (
	"errors"

	"github.com/udphttp-proxy/udphttp-proxy/internal/netpolicy"
)

type NetworkPolicies struct {
	Sources            netpolicy.Sources
	Playback, Playlist netpolicy.Prefixes
}

// NormalizeNetwork updates the configuration only after all lists have parsed.
func (c *Config) NormalizeNetwork() (NetworkPolicies, error) {
	sources, sourceErr := c.StreamPolicy().Compile()
	playback, playbackErr := netpolicy.Compile("streaming.allowed_client_networks", c.Streaming.AllowedClientNetworks, netpolicy.Client, false)
	playlist, playlistErr := netpolicy.Compile("playlist.allowed_client_networks", c.Playlist.AllowedClientNetworks, netpolicy.Client, false)
	binds, bindErr := netpolicy.Compile("network.http_bind_addresses", c.Network.HTTPBindAddresses, netpolicy.Bind, false)
	proxies, proxyErr := netpolicy.Compile("management.reverse_proxy.trusted_proxies", c.Management.ReverseProxy.TrustedProxies, netpolicy.Proxy, false)
	if err := errors.Join(sourceErr, playbackErr, playlistErr, bindErr, proxyErr); err != nil {
		return NetworkPolicies{}, err
	}
	c.Streaming.AllowedMulticastGroups = sources.Multicast.Values()
	c.Streaming.BlockedMulticastNetworks = sources.Blocked.Values()
	c.Streaming.AllowedUnicastSources = sources.Unicast.Values()
	c.Streaming.AllowedClientNetworks = playback.Values()
	c.Playlist.AllowedClientNetworks = playlist.Values()
	c.Network.HTTPBindAddresses = binds.Values()
	c.Management.ReverseProxy.TrustedProxies = proxies.Values()
	return NetworkPolicies{sources, playback, playlist}, nil
}
