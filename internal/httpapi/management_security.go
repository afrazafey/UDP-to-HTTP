package httpapi

import (
	"errors"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/udphttp-proxy/udphttp-proxy/internal/config"
)

// singleHeader rejects duplicate, empty, and whitespace-padded security headers.
func singleHeader(r *http.Request, name string) (string, error) {
	values := r.Header.Values(name)
	if len(values) == 0 {
		return "", nil
	}
	if len(values) != 1 || values[0] == "" || strings.TrimSpace(values[0]) != values[0] || strings.ContainsAny(values[0], "\r\n\x00") {
		return "", errors.New("Malformed management security header")
	}
	return values[0], nil
}

func localManagementHost(host string, port int) bool {
	u, err := config.ParseManagementOrigin("http://" + host)
	if err != nil {
		return false
	}
	actualPort := u.Port()
	if actualPort == "" {
		actualPort = "80"
	}
	ip, err := netip.ParseAddr(u.Hostname())
	return actualPort == strconv.Itoa(port) && (u.Hostname() == "localhost" || (err == nil && ip.Is4() && ip.IsLoopback()))
}

func (s *Service) authorizeManagement(r *http.Request) error {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	ip, ipErr := netip.ParseAddr(host)
	if err != nil || ipErr != nil || !ip.Is4() || !ip.IsLoopback() {
		return errors.New("Local administration only")
	}
	target := "http://" + r.Host
	proxy := s.options.Config.Management.ReverseProxy
	forwarded := false
	if proxy.Enabled {
		for name := range r.Header {
			name = strings.ToLower(name)
			if name == "forwarded" || strings.HasPrefix(name, "x-forwarded-") || name == "x-real-ip" {
				forwarded = true
				// Support one explicit X-Forwarded-* convention, never ambiguous
				// RFC Forwarded/legacy headers or a multi-proxy chain.
				if name != "x-forwarded-host" && name != "x-forwarded-proto" && name != "x-forwarded-for" && name != "x-forwarded-port" {
					return errors.New("Unsupported forwarded management header")
				}
			}
		}
	}
	if forwarded {
		if !slices.Contains(proxy.TrustedProxies, ip.String()) {
			return errors.New("Untrusted management proxy")
		}
		values := map[string]string{}
		for _, name := range []string{"X-Forwarded-Host", "X-Forwarded-Proto", "X-Forwarded-For", "X-Forwarded-Port"} {
			value, err := singleHeader(r, name)
			if err != nil {
				return err
			}
			values[name] = value
		}
		target = values["X-Forwarded-Proto"] + "://" + values["X-Forwarded-Host"]
		u, err := config.ParseManagementOrigin(target)
		if err != nil || !slices.Contains(proxy.TrustedOrigins, target) {
			return errors.New("Untrusted forwarded management origin")
		}
		client, err := netip.ParseAddr(values["X-Forwarded-For"])
		if err != nil || client.Zone() != "" || client.IsUnspecified() || client.IsMulticast() {
			return errors.New("Malformed forwarded client address")
		}
		if port := values["X-Forwarded-Port"]; port != "" {
			expected := u.Port()
			if expected == "" {
				expected = map[string]string{"http": "80", "https": "443"}[u.Scheme]
			}
			if port != expected {
				return errors.New("Inconsistent forwarded management port")
			}
		}
		// Nginx may replace Host with the upstream address or preserve the
		// external authority. No other Host is accepted, even from a proxy.
		if !localManagementHost(r.Host, s.options.Config.Management.Port) && r.Host != u.Host {
			return errors.New("Untrusted management host")
		}
	} else if !localManagementHost(r.Host, s.options.Config.Management.Port) {
		return errors.New("Untrusted management host")
	}

	origin, err := singleHeader(r, "Origin")
	if err != nil {
		return err
	}
	if origin != "" {
		if _, err := config.ParseManagementOrigin(origin); err != nil || origin != target {
			return errors.New("Cross-origin administration denied")
		}
	}
	referer, err := singleHeader(r, "Referer")
	if err != nil {
		return err
	}
	if referer != "" {
		u, err := url.Parse(referer)
		if err != nil || u.User != nil || strings.Contains(referer, "#") || u.Opaque != "" || u.Scheme+"://"+u.Host != target {
			return errors.New("Cross-origin administration denied")
		}
		if _, err := config.ParseManagementOrigin(u.Scheme + "://" + u.Host); err != nil {
			return errors.New("Malformed management referer")
		}
	}
	site, err := singleHeader(r, "Sec-Fetch-Site")
	if err != nil {
		return err
	}
	if site != "" && site != "same-origin" && site != "none" {
		return errors.New("Cross-site administration denied")
	}
	return nil
}
