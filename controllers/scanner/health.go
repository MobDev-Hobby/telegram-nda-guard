package scanner

import (
	"context"
	"time"

	"github.com/MobDev-Hobby/telegram-nda-guard/processors"
)

// Channel health ("traffic light") levels.
const (
	HealthGreen  = "green"
	HealthYellow = "yellow"
	HealthRed    = "red"
)

// Reasons behind a health level, for the Mini App to render.
const (
	HealthReasonOK         = "ok"
	HealthReasonNeverCheck = "never_checked"
	HealthReasonStale      = "stale"
	HealthReasonVeryStale  = "very_stale"
	HealthReasonViolations = "violations"
)

const (
	// healthStaleAfter turns a channel yellow when its last check is older.
	healthStaleAfter = 3 * 24 * time.Hour
	// healthCriticalAfter turns it red.
	healthCriticalAfter = 7 * 24 * time.Hour
)

// ChannelHealth summarises how well a channel is protected.
type ChannelHealth struct {
	Status    string                   `json:"status"`
	Reason    string                   `json:"reason"`
	LastCheck *processors.CheckSummary `json:"lastCheck,omitempty"`
}

// channelHealth: red when the last check found members without access or the
// channel has not been checked for a week; yellow when it has never been
// checked or not for three days; green otherwise.
func channelHealth(last *processors.CheckSummary, now time.Time) ChannelHealth {
	h := ChannelHealth{LastCheck: last}
	switch {
	case last == nil:
		h.Status, h.Reason = HealthYellow, HealthReasonNeverCheck
	case last.Bad > 0:
		h.Status, h.Reason = HealthRed, HealthReasonViolations
	case now.Sub(last.At) > healthCriticalAfter:
		h.Status, h.Reason = HealthRed, HealthReasonVeryStale
	case now.Sub(last.At) > healthStaleAfter:
		h.Status, h.Reason = HealthYellow, HealthReasonStale
	default:
		h.Status, h.Reason = HealthGreen, HealthReasonOK
	}
	return h
}

// recordCheck stores the outcome of a member check for the health indicator.
func (d *Domain) recordCheck(ctx context.Context, channelID int64, summary processors.CheckSummary) {
	err := d.updateProtectedChannel(ctx, channelID, func(pc *ProtectedChannel) {
		s := summary
		pc.LastCheck = &s
	})
	if err != nil {
		d.log.Errorf("can't record check of %d: %s", channelID, err)
	}
}
