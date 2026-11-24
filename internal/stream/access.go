package stream

import (
	"net/netip"

	"github.com/udphttp-proxy/udphttp-proxy/internal/netpolicy"
)

func (m *Manager) ClientAllowed(ip netip.Addr) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, allowed := m.playbackClients.Match(ip)
	return allowed
}

func (m *Manager) Sources() netpolicy.Sources {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sources
}

// UpdateAccess installs one policy for both source and client checks and waits
// for disallowed receivers to close. Allowed receivers keep their subscriptions.
func (m *Manager) UpdateAccess(sources netpolicy.Sources, clients netpolicy.Prefixes) {
	m.operations.Lock()
	defer m.operations.Unlock()
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return
	}
	m.sources, m.playbackClients = sources, clients
	var stopped []<-chan struct{}
	for _, e := range m.entries {
		wasBlocked := e.policyBlocked
		_, err := m.validate(e.channel)
		e.policyBlocked = err != nil
		if e.policyBlocked {
			if !wasBlocked {
				m.errorLocked(e, "policy", err)
			}
			e.state = "error"
			if w := e.worker; w != nil {
				w.stopping = true
				w.cancel()
				stopped = append(stopped, w.done)
			}
		} else if wasBlocked {
			e.state = "stopped"
			if !e.channel.Enabled {
				e.state = "disabled"
			} else if !e.blocked && !m.options.Disabled && e.channel.Startup == "always" {
				m.startLocked(e, true)
			}
		}
		for _, sub := range e.clients {
			ip, _ := netip.ParseAddr(sub.remoteIP)
			if _, allowed := clients.Match(ip); e.policyBlocked || !allowed {
				m.terminateLocked(sub)
			}
		}
	}
	m.mu.Unlock()
	for _, done := range stopped {
		<-done
	}
}
