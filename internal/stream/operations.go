package stream

func (m *Manager) ReceiverCount() int { m.mu.Lock(); defer m.mu.Unlock(); return len(m.receivers) }
func (m *Manager) Disconnect(id uint64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, e := range m.entries {
		if sub := e.clients[id]; sub != nil {
			m.terminateLocked(sub)
			return nil
		}
	}
	return ErrNotFound
}
