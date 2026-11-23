package stream

import (
	"context"
	"errors"
	"fmt"
)

// Batch validates the complete candidate before atomically persisting it. Running
// receivers are replaced only after commit, under the existing operation lock.
func (m *Manager) Batch(ctx context.Context, upserts []Channel, deletes []string) error {
	return m.batch(ctx, upserts, deletes, true)
}

// ImportChannels checks create-versus-update intent while holding the same
// operation lock as commit, preventing preview/commit races from overwriting IDs.
func (m *Manager) ImportChannels(ctx context.Context, channels []Channel, allowUpdates bool) error {
	return m.batch(ctx, channels, nil, allowUpdates)
}
func (m *Manager) batch(ctx context.Context, upserts []Channel, deletes []string, allowUpdates bool) error {
	m.operations.Lock()
	defer m.operations.Unlock()
	return m.batchLocked(ctx, upserts, deletes, allowUpdates)
}

func (m *Manager) BulkChannels(ctx context.Context, ids []string, action string) error {
	if len(ids) == 0 || len(ids) > m.options.MaxStreams {
		return ErrLimit
	}
	if action != "enable" && action != "disable" && action != "delete" {
		return errors.New("bulk action must be enable, disable or delete")
	}
	m.operations.Lock()
	defer m.operations.Unlock()
	upserts := []Channel{}
	deletes := []string{}
	m.mu.Lock()
	for _, id := range ids {
		e := m.entries[id]
		if e == nil {
			m.mu.Unlock()
			return ErrNotFound
		}
		if action == "delete" {
			deletes = append(deletes, id)
		} else {
			c := e.channel
			c.Enabled = action == "enable"
			upserts = append(upserts, c)
		}
	}
	m.mu.Unlock()
	return m.batchLocked(ctx, upserts, deletes, true)
}

// Caller owns the operation lock, including any intent-specific snapshot.
func (m *Manager) batchLocked(ctx context.Context, upserts []Channel, deletes []string, allowUpdates bool) error {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return ErrClosed
	}
	candidate := make(map[string]Channel, len(m.entries))
	for id, e := range m.entries {
		candidate[id] = e.channel
	}
	m.mu.Unlock()
	seen := map[string]bool{}
	upserts = append([]Channel(nil), upserts...)
	for _, id := range deletes {
		if seen[id] {
			return ErrConflict
		}
		seen[id] = true
		if _, ok := candidate[id]; !ok {
			return ErrNotFound
		}
		delete(candidate, id)
	}
	for i, c := range upserts {
		if _, exists := candidate[c.ID]; exists && !allowUpdates {
			return fmt.Errorf("%w: existing imports require explicit update_existing confirmation", ErrConflict)
		}
		if seen[c.ID] {
			return ErrConflict
		}
		seen[c.ID] = true
		old, exists := candidate[c.ID]
		old.Enabled = false
		if exists && !c.Enabled && old == c {
			upserts[i] = c
			candidate[c.ID] = c
			continue
		}
		v, err := m.validate(c)
		if err != nil {
			return fmt.Errorf("channel %s: %w", c.ID, err)
		}
		if _, _, err = resolveInterface(v.Interface, v.SourceType == "multicast"); err != nil {
			return err
		}
		upserts[i] = v
		candidate[v.ID] = v
	}
	if len(candidate) > m.options.MaxStreams {
		return ErrLimit
	}
	keys := map[string]string{}
	binds := map[string]bool{}
	for _, c := range candidate {
		key := c.SourceKey()
		if id, exists := keys[key]; exists && id != c.ID {
			return ErrConflict
		}
		keys[key] = c.ID
		if c.SourceType == "unicast" {
			selection := c.Interface
			iface, ip, err := resolveInterface(c.Interface, false)
			if err == nil {
				selection = iface.Name + "@" + ip.String()
			}
			bind := fmt.Sprintf("%s:%d", selection, c.SourcePort)
			if binds[bind] {
				return ErrConflict
			}
			binds[bind] = true
		}
	}
	if m.options.Repository != nil {
		repo, ok := m.options.Repository.(interface {
			ApplyChannels(context.Context, []Channel, []string) error
		})
		if !ok {
			return errors.New("repository does not support atomic batch operations")
		}
		if err := repo.ApplyChannels(ctx, upserts, deletes); err != nil {
			return fmt.Errorf("%w: %v", ErrStorage, err)
		}
	}
	// Cancel every affected receiver before joining any of them, so a large
	// batch pays one read-deadline interval rather than one interval per source.
	m.mu.Lock()
	for id := range seen {
		if e := m.entries[id]; e != nil {
			e.blocked = true
			if e.worker != nil {
				e.worker.cancel()
			}
		}
	}
	m.mu.Unlock()
	for id := range seen {
		m.mu.Lock()
		old := m.entries[id]
		m.mu.Unlock()
		if old != nil {
			m.stop(old, true)
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, id := range deletes {
		delete(m.entries, id)
	}
	for _, c := range upserts {
		e := newEntry(c)
		m.entries[c.ID] = e
		if c.Enabled && c.Startup == "always" && !m.options.Disabled {
			m.startLocked(e, true)
		}
	}
	return nil
}

func ValidateInterface(c Channel) error {
	_, _, err := resolveInterface(c.Interface, c.SourceType == "multicast")
	return err
}
