package stream

import (
	"math/rand/v2"
	"time"
)

type RecoveryPolicy struct {
	Enabled                         bool
	MinDelay, MaxDelay, RejoinAfter time.Duration
	Jitter                          float64
	MaxAttempts                     int
}

func DefaultRecovery() RecoveryPolicy {
	return RecoveryPolicy{Enabled: true, MinDelay: 250 * time.Millisecond, MaxDelay: 30 * time.Second, RejoinAfter: 30 * time.Second, Jitter: 0.2}
}
func (p RecoveryPolicy) delay(base time.Duration) time.Duration {
	return max(p.MinDelay, min(p.MaxDelay, time.Duration(float64(base)*(1-p.Jitter+rand.Float64()*2*p.Jitter))))
}
func (m *Manager) healthLocked(e *entry) string {
	if !e.channel.Enabled || m.options.Disabled {
		return "disabled"
	}
	switch e.state {
	case "running":
		return "healthy"
	case "retrying":
		return "recovering"
	case "starting", "waiting":
		if !e.lastDatagram.IsZero() && time.Since(e.lastDatagram) < m.options.InactivityTimeout {
			return "degraded"
		}
		return "waiting_for_source"
	default:
		return "offline"
	}
}
func timestamp(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	utc := value.UTC()
	return &utc
}
