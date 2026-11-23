package config

import (
	"errors"
	"fmt"
	"net/netip"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

var originDNSLabel = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)

// ParseManagementOrigin accepts a serialized HTTP(S) origin, never a URL path,
// wildcard, credentials, or a list. Matching uses its exact serialized spelling.
func ParseManagementOrigin(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || raw != u.Scheme+"://"+u.Host {
		return nil, errors.New("expected an exact http(s) origin without a path, credentials, query or fragment")
	}
	host := u.Hostname()
	if ip, err := netip.ParseAddr(host); err == nil {
		if ip.Zone() != "" || ip.IsUnspecified() || ip.IsMulticast() || ip.String() != host {
			return nil, errors.New("origin IP must be canonical unicast")
		}
	} else {
		if len(host) == 0 || len(host) > 253 || numericAddress.MatchString(host) {
			return nil, errors.New("invalid origin hostname")
		}
		for _, label := range strings.Split(host, ".") {
			if !originDNSLabel.MatchString(label) {
				return nil, errors.New("origin hostname must use lowercase DNS labels or a literal IP")
			}
		}
	}
	port := u.Port()
	if port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || !validPort(n) || strconv.Itoa(n) != port {
			return nil, errors.New("invalid origin port")
		}
	} else if strings.HasSuffix(u.Host, ":") {
		return nil, errors.New("empty origin port")
	}
	authority := host
	if strings.Contains(host, ":") {
		authority = "[" + host + "]"
	}
	if port != "" {
		authority += ":" + port
	}
	if u.Host != authority {
		return nil, errors.New("invalid origin authority")
	}
	return u, nil
}

func (p ManagementReverseProxy) Validate() error {
	if p.Enabled && (len(p.TrustedProxies) == 0 || len(p.TrustedOrigins) == 0) {
		return errors.New("management.reverse_proxy requires trusted_proxies and trusted_origins when enabled")
	}
	if len(p.TrustedProxies) > 16 || len(p.TrustedOrigins) > 32 {
		return errors.New("management.reverse_proxy allows at most 16 proxies and 32 origins")
	}
	seen := map[string]bool{}
	for _, address := range p.TrustedProxies {
		ip, err := netip.ParseAddr(address)
		// Management only listens on IPv4 loopback, so a same-host proxy must
		// connect from a literal IPv4 loopback address, never a LAN IP or CIDR.
		if err != nil || !ip.Is4() || !ip.IsLoopback() || seen[address] {
			return errors.New("management.reverse_proxy.trusted_proxies requires unique literal IPv4 loopback addresses")
		}
		seen[address] = true
	}
	seen = map[string]bool{}
	for _, origin := range p.TrustedOrigins {
		if _, err := ParseManagementOrigin(origin); err != nil {
			return fmt.Errorf("management.reverse_proxy.trusted_origins: %w", err)
		}
		if seen[origin] {
			return errors.New("duplicate management.reverse_proxy.trusted_origins entry")
		}
		seen[origin] = true
	}
	return nil
}
