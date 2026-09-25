package scanner

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	guard "github.com/MobDev-Hobby/telegram-nda-guard"
	"github.com/MobDev-Hobby/telegram-nda-guard/storage/whitelist"
)

const (
	// DefaultWhitelistTTL is how long an approval lasts before a channel
	// administrator has to re-approve it.
	DefaultWhitelistTTL = 30 * 24 * time.Hour
	// DefaultWhitelistRemindBefore is how early control chats are reminded
	// about an approval running out.
	DefaultWhitelistRemindBefore = 3 * 24 * time.Hour

	whitelistReminderInterval = time.Hour
	maxWhitelistBatch         = 200
)

var (
	ErrWhitelistDisabled = errors.New("whitelist is not configured")
	ErrNotWhitelisted    = errors.New("user is not in the whitelist")
)

// WhitelistStorage persists per-channel whitelists.
// storage/whitelist/redis.Domain implements it.
type WhitelistStorage interface {
	LoadWhitelist(ctx context.Context, channelID int64) ([]whitelist.Entry, error)
	StoreWhitelistEntry(ctx context.Context, channelID int64, entry whitelist.Entry) error
	DropWhitelistEntry(ctx context.Context, channelID, userID int64) error
}

// WhitelistEntryView is a whitelist entry as shown in the Mini App.
type WhitelistEntryView struct {
	whitelist.Entry
	Active bool `json:"active"`
}

// whitelistChecker lets active whitelist entries of a channel through without
// asking the wrapped checker. Expired entries fall through to the normal
// check: that is what forces the periodic re-approval.
type whitelistChecker struct {
	d         *Domain
	channelID int64
	next      CheckUserAccess
}

func (c whitelistChecker) HasAccess(ctx context.Context, user *guard.User) (bool, error) {
	if _, ok := c.d.activeWhitelistEntry(ctx, c.channelID, user.ID); ok {
		return true, nil
	}
	return c.next.HasAccess(ctx, user)
}

// withWhitelist wraps checker with the channel's whitelist when one is
// configured.
func (d *Domain) withWhitelist(channelID int64, checker CheckUserAccess) CheckUserAccess {
	if d.whitelistStorage == nil || checker == nil {
		return checker
	}
	return whitelistChecker{d: d, channelID: channelID, next: checker}
}

// channelWhitelist returns the channel's entries, loading them from storage
// on first use. The in-memory copy is the source of truth afterwards; every
// change is written through.
func (d *Domain) channelWhitelist(ctx context.Context, channelID int64) (map[int64]whitelist.Entry, error) {
	d.whitelistMutex.Lock()
	defer d.whitelistMutex.Unlock()
	if entries, ok := d.whitelists[channelID]; ok {
		return entries, nil
	}
	loaded, err := d.whitelistStorage.LoadWhitelist(ctx, channelID)
	if err != nil {
		return nil, fmt.Errorf("load whitelist of %d: %w", channelID, err)
	}
	entries := make(map[int64]whitelist.Entry, len(loaded))
	for _, e := range loaded {
		entries[e.UserID] = e
	}
	d.whitelists[channelID] = entries
	return entries, nil
}

func (d *Domain) activeWhitelistEntry(ctx context.Context, channelID, userID int64) (whitelist.Entry, bool) {
	if d.whitelistStorage == nil {
		return whitelist.Entry{}, false
	}
	if _, err := d.channelWhitelist(ctx, channelID); err != nil {
		// Fail closed: without the list nobody is whitelisted, users go
		// through the regular check.
		d.log.Errorf("%s", err)
		return whitelist.Entry{}, false
	}
	d.whitelistMutex.Lock()
	entry, ok := d.whitelists[channelID][userID]
	d.whitelistMutex.Unlock()
	if !ok || !entry.Active(d.now()) {
		return whitelist.Entry{}, false
	}
	return entry, true
}

// putWhitelistEntry persists entry and updates the in-memory copy.
func (d *Domain) putWhitelistEntry(ctx context.Context, channelID int64, entry whitelist.Entry) error {
	if _, err := d.channelWhitelist(ctx, channelID); err != nil {
		return err
	}
	storeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := d.whitelistStorage.StoreWhitelistEntry(storeCtx, channelID, entry); err != nil {
		return fmt.Errorf("persist whitelist entry: %w", err)
	}
	d.whitelistMutex.Lock()
	d.whitelists[channelID][entry.UserID] = entry
	d.whitelistMutex.Unlock()
	return nil
}

// ListWhitelist implements MiniAppService.
func (d *Domain) ListWhitelist(ctx context.Context, channelID int64) ([]WhitelistEntryView, error) {
	if d.whitelistStorage == nil {
		return nil, ErrWhitelistDisabled
	}
	if _, ok := d.getProtectedChannel(channelID); !ok {
		return nil, fmt.Errorf("channel %d is not protected", channelID)
	}
	if _, err := d.channelWhitelist(ctx, channelID); err != nil {
		return nil, err
	}
	now := d.now()
	d.whitelistMutex.Lock()
	out := make([]WhitelistEntryView, 0, len(d.whitelists[channelID]))
	for _, e := range d.whitelists[channelID] {
		out = append(out, WhitelistEntryView{Entry: e, Active: e.Active(now)})
	}
	d.whitelistMutex.Unlock()
	// Soonest to expire first: that is what needs attention.
	sort.Slice(out, func(i, j int) bool { return out[i].ExpiresAt.Before(out[j].ExpiresAt) })
	return out, nil
}

// AddToWhitelist implements MiniAppService. Users are taken from a scan of the
// same channel, which is where their names and IDs come from.
func (d *Domain) AddToWhitelist(
	ctx context.Context,
	channelID int64,
	scanID string,
	userIDs []int64,
	callerID int64,
) ([]WhitelistEntryView, error) {
	if d.whitelistStorage == nil {
		return nil, ErrWhitelistDisabled
	}
	if len(userIDs) > maxWhitelistBatch {
		return nil, fmt.Errorf("at most %d users can be whitelisted at once", maxWhitelistBatch)
	}
	pc, ok := d.getProtectedChannel(channelID)
	if !ok {
		return nil, fmt.Errorf("channel %d is not protected", channelID)
	}

	d.scansMutex.Lock()
	job, ok := d.scans[scanID]
	if !ok || job.view.ChannelID != channelID {
		d.scansMutex.Unlock()
		return nil, ErrScanNotFound
	}
	byID := make(map[int64]ScannedUser, len(job.view.Users))
	for _, u := range job.view.Users {
		byID[u.ID] = u
	}
	d.scansMutex.Unlock()

	now := d.now()
	added := make([]WhitelistEntryView, 0, len(userIDs))
	for _, id := range userIDs {
		u, found := byID[id]
		if !found {
			return nil, fmt.Errorf("user %d is not in this scan", id)
		}
		if u.Protected && u.Note != noteWhitelisted {
			return nil, fmt.Errorf("user %d is %s and needs no whitelist", id, u.Note)
		}
		entry := whitelist.Entry{
			UserID:     u.ID,
			FirstName:  u.FirstName,
			LastName:   u.LastName,
			Username:   u.Username,
			ApprovedBy: callerID,
			ApprovedAt: now,
			ExpiresAt:  now.Add(d.whitelistTTL),
		}
		if err := d.putWhitelistEntry(ctx, channelID, entry); err != nil {
			return nil, err
		}
		added = append(added, WhitelistEntryView{Entry: entry, Active: true})
	}

	// Reflect it in the scan the admin is looking at.
	d.scansMutex.Lock()
	if job, ok := d.scans[scanID]; ok {
		for i := range job.view.Users {
			for _, e := range added {
				if job.view.Users[i].ID == e.UserID {
					until := e.ExpiresAt
					job.view.Users[i].Status = UserStatusGood
					job.view.Users[i].Protected = true
					job.view.Users[i].Note = noteWhitelisted
					job.view.Users[i].WhitelistedUntil = &until
				}
			}
		}
	}
	d.scansMutex.Unlock()

	names := make([]string, 0, len(added))
	for _, e := range added {
		names = append(names, entryName(e.Entry))
	}
	d.notifyControlChats(ctx, pc, fmt.Sprintf(
		"<b>Whitelist of %s</b>\n\n%s added by <a href=\"tg://user?id=%d\">%d</a> until %s.",
		d.channelTitle(channelID), strings.Join(names, ", "), callerID, callerID, formatDate(now.Add(d.whitelistTTL)),
	))
	return added, nil
}

// RenewWhitelistEntry implements MiniAppService: a channel administrator
// re-approves the user for another period.
func (d *Domain) RenewWhitelistEntry(ctx context.Context, channelID, userID, callerID int64) (WhitelistEntryView, error) {
	if d.whitelistStorage == nil {
		return WhitelistEntryView{}, ErrWhitelistDisabled
	}
	pc, ok := d.getProtectedChannel(channelID)
	if !ok {
		return WhitelistEntryView{}, fmt.Errorf("channel %d is not protected", channelID)
	}
	entries, err := d.channelWhitelist(ctx, channelID)
	if err != nil {
		return WhitelistEntryView{}, err
	}
	d.whitelistMutex.Lock()
	entry, found := entries[userID]
	d.whitelistMutex.Unlock()
	if !found {
		return WhitelistEntryView{}, ErrNotWhitelisted
	}
	now := d.now()
	entry.ApprovedBy = callerID
	entry.ApprovedAt = now
	entry.ExpiresAt = now.Add(d.whitelistTTL)
	entry.ExpiryNotified = false
	if err := d.putWhitelistEntry(ctx, channelID, entry); err != nil {
		return WhitelistEntryView{}, err
	}
	d.notifyControlChats(ctx, pc, fmt.Sprintf(
		"<b>Whitelist of %s</b>\n\n%s re-approved by <a href=\"tg://user?id=%d\">%d</a> until %s.",
		d.channelTitle(channelID), entryName(entry), callerID, callerID, formatDate(entry.ExpiresAt),
	))
	return WhitelistEntryView{Entry: entry, Active: true}, nil
}

// RemoveWhitelistEntry implements MiniAppService.
func (d *Domain) RemoveWhitelistEntry(ctx context.Context, channelID, userID, callerID int64) error {
	if d.whitelistStorage == nil {
		return ErrWhitelistDisabled
	}
	pc, ok := d.getProtectedChannel(channelID)
	if !ok {
		return fmt.Errorf("channel %d is not protected", channelID)
	}
	entries, err := d.channelWhitelist(ctx, channelID)
	if err != nil {
		return err
	}
	d.whitelistMutex.Lock()
	entry, found := entries[userID]
	d.whitelistMutex.Unlock()
	if !found {
		return ErrNotWhitelisted
	}
	storeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := d.whitelistStorage.DropWhitelistEntry(storeCtx, channelID, userID); err != nil {
		return fmt.Errorf("drop whitelist entry: %w", err)
	}
	d.whitelistMutex.Lock()
	delete(d.whitelists[channelID], userID)
	d.whitelistMutex.Unlock()
	d.notifyControlChats(ctx, pc, fmt.Sprintf(
		"<b>Whitelist of %s</b>\n\n%s removed by <a href=\"tg://user?id=%d\">%d</a>.",
		d.channelTitle(channelID), entryName(entry), callerID, callerID,
	))
	return nil
}

// RunWhitelistReminders periodically reminds control chats about approvals
// that are about to run out and tells them when one has expired.
func (d *Domain) RunWhitelistReminders(ctx context.Context) {
	if d.whitelistStorage == nil {
		return
	}
	go func() {
		ticker := time.NewTicker(whitelistReminderInterval)
		defer ticker.Stop()
		for {
			d.remindWhitelists(ctx)
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

func (d *Domain) remindWhitelists(ctx context.Context) {
	now := d.now()
	d.channelsMutex.RLock()
	protected := make([]ProtectedChannel, 0, len(d.protectedChannels))
	for _, pc := range d.protectedChannels {
		protected = append(protected, pc)
	}
	d.channelsMutex.RUnlock()

	for _, pc := range protected {
		entries, err := d.channelWhitelist(ctx, pc.ID)
		if err != nil {
			d.log.Errorf("%s", err)
			continue
		}
		d.whitelistMutex.Lock()
		var expiring, expired []whitelist.Entry
		for _, e := range entries {
			switch {
			case !e.Active(now) && !e.ExpiryNotified:
				expired = append(expired, e)
			case e.Active(now) && e.ExpiresAt.Sub(now) <= d.whitelistRemindBefore && !e.RemindedFor.Equal(e.ExpiresAt):
				expiring = append(expiring, e)
			}
		}
		d.whitelistMutex.Unlock()
		if len(expiring) == 0 && len(expired) == 0 {
			continue
		}

		var text strings.Builder
		fmt.Fprintf(&text, "<b>Whitelist of %s</b>\n", d.channelTitle(pc.ID))
		if len(expiring) > 0 {
			text.WriteString("\nApproval runs out soon, re-approve to keep access:\n")
			for _, e := range expiring {
				fmt.Fprintf(&text, "• %s — until %s\n", entryName(e), formatDate(e.ExpiresAt))
			}
		}
		if len(expired) > 0 {
			text.WriteString("\nApproval expired, these users are checked as usual again:\n")
			for _, e := range expired {
				fmt.Fprintf(&text, "• %s\n", entryName(e))
			}
		}
		if d.miniAppURL != "" {
			text.WriteString("\nOpen the app with /app → Whitelist.")
		}
		if !d.notifyControlChats(ctx, pc, text.String()) {
			// Retry on the next tick instead of marking as sent.
			continue
		}

		for _, e := range expiring {
			e.RemindedFor = e.ExpiresAt
			if err := d.putWhitelistEntry(ctx, pc.ID, e); err != nil {
				d.log.Errorf("%s", err)
			}
		}
		for _, e := range expired {
			e.ExpiryNotified = true
			if err := d.putWhitelistEntry(ctx, pc.ID, e); err != nil {
				d.log.Errorf("%s", err)
			}
		}
	}
}

// notifyControlChats sends text to the channel's control chats, with a
// button opening the channel in the Mini App when a direct link is possible.
// It reports whether at least one chat got the message.
func (d *Domain) notifyControlChats(ctx context.Context, pc ProtectedChannel, text string) bool {
	msg := guard.Message{Text: text}
	if d.miniAppShortName != "" && d.telegramBot.Username() != "" {
		msg.InlineButtons = [][]guard.InlineButton{{{
			Text: "Open NDA Guard",
			URL:  fmt.Sprintf("https://t.me/%s/%s?startapp=%d", d.telegramBot.Username(), d.miniAppShortName, pc.ID),
		}}}
	}
	sent := false
	for _, chatID := range pc.CommandChannelIDs {
		m := msg
		m.ChatID = chatID
		if err := d.telegramBot.SendMessage(ctx, &m); err != nil {
			d.log.Errorf("can't notify control chat %d: %s", chatID, err)
			continue
		}
		sent = true
	}
	return sent
}

func (d *Domain) channelTitle(channelID int64) string {
	if ch, ok := d.getChannel(channelID); ok && ch.title != "" {
		return ch.title
	}
	return fmt.Sprintf("%d", channelID)
}

func entryName(e whitelist.Entry) string {
	name := strings.TrimSpace(e.FirstName + " " + e.LastName)
	if name == "" {
		name = fmt.Sprintf("%d", e.UserID)
	}
	if e.Username != "" {
		name += " (@" + e.Username + ")"
	}
	return name
}

func formatDate(t time.Time) string {
	return t.UTC().Format("02.01.2006")
}
