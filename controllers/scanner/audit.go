package scanner

import (
	"context"
	"strings"
	"time"

	"github.com/MobDev-Hobby/telegram-nda-guard/storage/audit"
)

// AuditStorage keeps each channel's action log.
// storage/audit/redis.Domain implements it.
type AuditStorage interface {
	AppendAudit(ctx context.Context, channelID int64, event audit.Event) error
	ListAudit(ctx context.Context, channelID int64, limit int) ([]audit.Event, error)
}

const maxAuditPage = 200

// recordAudit appends event to the channel's log (when a log is configured)
// and, when chatText is set, posts it to the channel's control chats. Logging
// failures never fail the action being logged.
func (d *Domain) recordAudit(ctx context.Context, pc ProtectedChannel, event audit.Event, chatText string) {
	event.At = d.now()
	if event.ActorID != 0 && event.ActorName == "" {
		event.ActorName = d.userName(event.ActorID)
	}
	if d.auditStorage != nil {
		storeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		if err := d.auditStorage.AppendAudit(storeCtx, pc.ID, event); err != nil {
			d.log.Errorf("can't write audit event %s for %d: %s", event.Action, pc.ID, err)
		}
		cancel()
	}
	if chatText != "" {
		d.notifyControlChats(ctx, pc, chatText)
	}
}

// ListAudit implements MiniAppService: the channel's most recent events,
// newest first.
func (d *Domain) ListAudit(ctx context.Context, channelID int64, limit int) ([]audit.Event, error) {
	if d.auditStorage == nil {
		return []audit.Event{}, nil
	}
	if limit <= 0 || limit > maxAuditPage {
		limit = maxAuditPage
	}
	events, err := d.auditStorage.ListAudit(ctx, channelID, limit)
	if err != nil {
		return nil, err
	}
	if events == nil {
		events = []audit.Event{}
	}
	return events, nil
}

// NoteUserName remembers a display name for userID so logs and chat messages
// can show a name instead of a bare ID. Fed from Mini App launch data.
func (d *Domain) NoteUserName(userID int64, name string) {
	name = strings.TrimSpace(name)
	if userID == 0 || name == "" {
		return
	}
	d.namesMutex.Lock()
	d.userNames[userID] = name
	d.namesMutex.Unlock()
}

func (d *Domain) userName(userID int64) string {
	d.namesMutex.Lock()
	defer d.namesMutex.Unlock()
	return d.userNames[userID]
}
