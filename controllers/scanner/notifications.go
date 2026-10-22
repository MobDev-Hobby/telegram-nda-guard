package scanner

import (
	"context"
	"errors"
	"fmt"
	"html"
	"time"

	"github.com/MobDev-Hobby/telegram-nda-guard/processors"
)

const (
	NotificationAll               = "all"
	NotificationViolationsSummary = "violations_summary"
	NotificationViolationsOnly    = "violations_only"
)

func notificationOptions(opts *processors.NotificationOptions) processors.NotificationOptions {
	out := processors.NotificationOptions{Mode: NotificationViolationsSummary, IntervalHours: 24}
	if opts != nil {
		out = *opts
	}
	return out
}

func validateNotifications(opts *processors.NotificationOptions) error {
	if opts == nil {
		return nil
	}
	switch opts.Mode {
	case NotificationAll, NotificationViolationsSummary, NotificationViolationsOnly:
	default:
		return errors.New("invalid notification mode")
	}
	if opts.IntervalHours < 1 || opts.IntervalHours > 168 {
		return errors.New("summary interval must be between 1 and 168 hours")
	}
	return nil
}

// Reserve the next scheduled summary atomically and persist its timestamp.
// Clean processors still run with an empty recipient list when a summary is muted.
func (d *Domain) scheduledReport(ctx context.Context, pc ProtectedChannel, report *processors.AccessReport) {
	opts := notificationOptions(pc.Notifications)
	report.ViolationsOnly = opts.Mode == NotificationViolationsOnly
	issues := len(report.DeniedUsers) > 0 || len(report.UnknownUsers) > 0 || report.Stats.Partial()
	if opts.Mode != NotificationAll && !issues {
		report.ReportChannels = nil
		return
	}
	due := false
	err := d.updateProtectedChannel(ctx, pc.ID, func(current *ProtectedChannel) {
		if current.LastNotifiedAt.IsZero() || d.now().Sub(current.LastNotifiedAt) >= time.Duration(opts.IntervalHours)*time.Hour {
			current.LastNotifiedAt = d.now()
			due = true
		}
	})
	if err != nil {
		d.log.Errorf("can't reserve notification for %d: %s", pc.ID, err)
	}
	if !due || err != nil {
		report.ReportChannels = nil
	}
}

// Throttle ongoing rights failures, which previously pinged every rights check.
func (d *Domain) notifyRightsIssue(ctx context.Context, pc ProtectedChannel, kind, description string) {
	opts := notificationOptions(pc.Notifications)
	d.notificationMutex.Lock()
	if d.noticeTimes[pc.ID] == nil {
		d.noticeTimes[pc.ID] = map[string]time.Time{}
	}
	last := d.noticeTimes[pc.ID][kind]
	if !last.IsZero() && d.now().Sub(last) < time.Duration(opts.IntervalHours)*time.Hour {
		d.notificationMutex.Unlock()
		return
	}
	d.noticeTimes[pc.ID][kind] = d.now()
	d.notificationMutex.Unlock()
	title := d.channelTitle(pc.ID)
	who := "unknown (added before attribution was recorded)"
	if pc.AddedBy != 0 {
		who = actorLink(pc.AddedBy, pc.AddedByName)
	}
	message := fmt.Sprintf("<b>%s</b> (ID %d)\n%s\nAdded by: %s", html.EscapeString(title), pc.ID, description, who)
	if !d.notifyControlChats(ctx, pc, message) {
		d.NotifyAdmin(ctx, message)
	}
}
