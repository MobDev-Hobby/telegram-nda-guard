package scanner

import (
	"context"
	"errors"
	"fmt"
	"html"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	guard "github.com/MobDev-Hobby/telegram-nda-guard"
	"github.com/MobDev-Hobby/telegram-nda-guard/storage/audit"
	"github.com/MobDev-Hobby/telegram-nda-guard/storage/whitelist"
)

const (
	// DefaultWhitelistTTL is how long an approval lasts before a channel
	// administrator has to review it again.
	DefaultWhitelistTTL = 30 * 24 * time.Hour
	// DefaultWhitelistRemindBefore is how early managers are reminded about an
	// approval running out.
	DefaultWhitelistRemindBefore = 3 * 24 * time.Hour
	// whitelistExpiredRemindEvery is how often an expired approval is
	// reminded about until someone reviews it.
	whitelistExpiredRemindEvery = 24 * time.Hour

	whitelistReminderInterval = time.Hour
	maxWhitelistBatch         = 200
	// MaxWhitelistNoteLen caps the approver's note, in characters.
	MaxWhitelistNoteLen = 500
	// MaxWhitelistEntryTTL caps a temporary approval.
	MaxWhitelistEntryTTL = 365 * 24 * time.Hour
)

var (
	ErrWhitelistDisabled = errors.New("whitelist is not configured")
	ErrNotWhitelisted    = errors.New("user is not in the whitelist")
	ErrNoteTooLong       = fmt.Errorf("the note is longer than %d characters", MaxWhitelistNoteLen)
	ErrBadEntryTTL       = errors.New("the approval term must be between 1 hour and 365 days")
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
	// Expired entries still protect the user but need a review.
	Expired bool `json:"expired"`
}

// whitelistChecker lets whitelisted users of a channel through without asking
// the wrapped checker. Expired entries still count: an overdue review must not
// get someone removed; it only triggers daily reminders.
type whitelistChecker struct {
	d         *Domain
	channelID int64
	next      CheckUserAccess
}

func (c whitelistChecker) HasAccess(ctx context.Context, user *guard.User) (bool, error) {
	if _, ok := c.d.whitelistEntry(ctx, c.channelID, user.ID); ok {
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
	entries, ok := d.whitelists[channelID]
	d.whitelistMutex.Unlock()
	if ok {
		return entries, nil
	}

	// Load outside the lock (it's shared by all channels) and with a
	// deadline, so a slow Redis doesn't stall every check.
	loadCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	loaded, err := d.whitelistStorage.LoadWhitelist(loadCtx, channelID)
	if err != nil {
		return nil, fmt.Errorf("load whitelist of %d: %w", channelID, err)
	}
	entries = make(map[int64]whitelist.Entry, len(loaded))
	for _, e := range loaded {
		entries[e.UserID] = e
	}

	d.whitelistMutex.Lock()
	defer d.whitelistMutex.Unlock()
	if current, ok := d.whitelists[channelID]; ok {
		// Someone else loaded it meanwhile and may have changed it since.
		return current, nil
	}
	d.whitelists[channelID] = entries
	return entries, nil
}

// whitelistEntry returns the user's entry, expired or not.
func (d *Domain) whitelistEntry(ctx context.Context, channelID, userID int64) (whitelist.Entry, bool) {
	if d.whitelistStorage == nil {
		return whitelist.Entry{}, false
	}
	if _, err := d.channelWhitelist(ctx, channelID); err != nil {
		// Without the list nobody is whitelisted: users go through the
		// regular check.
		d.log.Errorf("%s", err)
		return whitelist.Entry{}, false
	}
	d.whitelistMutex.Lock()
	entry, ok := d.whitelists[channelID][userID]
	d.whitelistMutex.Unlock()
	if ok && entry.Deleted(d.now()) {
		// Past a temporary approval's end the entry no longer applies, even
		// before the reminder loop removes it.
		return whitelist.Entry{}, false
	}
	return entry, ok
}

// deleteAt turns a requested term into an entry end; 0 means none.
func (d *Domain) deleteAt(ttl time.Duration, now time.Time) (*time.Time, error) {
	if ttl == 0 {
		return nil, nil
	}
	if ttl < time.Hour || ttl > MaxWhitelistEntryTTL {
		return nil, ErrBadEntryTTL
	}
	at := now.Add(ttl)
	return &at, nil
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

func (d *Domain) entryView(e whitelist.Entry) WhitelistEntryView {
	return WhitelistEntryView{Entry: e, Expired: e.Expired(d.now())}
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
	d.whitelistMutex.Lock()
	out := make([]WhitelistEntryView, 0, len(d.whitelists[channelID]))
	for _, e := range d.whitelists[channelID] {
		out = append(out, d.entryView(e))
	}
	d.whitelistMutex.Unlock()
	// Soonest to expire (or longest overdue) first: that needs attention.
	sort.Slice(out, func(i, j int) bool { return out[i].ExpiresAt.Before(out[j].ExpiresAt) })
	return out, nil
}

func cleanNote(note string) (string, error) {
	note = strings.TrimSpace(note)
	if utf8.RuneCountInString(note) > MaxWhitelistNoteLen {
		return "", ErrNoteTooLong
	}
	return note, nil
}

// AddToWhitelist implements MiniAppService. Users are taken from a scan of the
// same channel, which is where their names and IDs come from. note is the
// approver's reason, stored with every entry.
func (d *Domain) AddToWhitelist(
	ctx context.Context,
	channelID int64,
	scanID string,
	userIDs []int64,
	note string,
	ttl time.Duration,
	callerID int64,
) ([]WhitelistEntryView, error) {
	if d.whitelistStorage == nil {
		return nil, ErrWhitelistDisabled
	}
	if len(userIDs) > maxWhitelistBatch {
		return nil, fmt.Errorf("at most %d users can be whitelisted at once", maxWhitelistBatch)
	}
	note, err := cleanNote(note)
	if err != nil {
		return nil, err
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

	// Validate the whole batch before writing anything.
	for _, id := range userIDs {
		u, found := byID[id]
		if !found {
			return nil, fmt.Errorf("user %d is not in this scan", id)
		}
		if u.Protected && u.Note != noteWhitelisted {
			return nil, fmt.Errorf("user %d is %s and needs no whitelist", id, u.Note)
		}
	}

	now := d.now()
	end, err := d.deleteAt(ttl, now)
	if err != nil {
		return nil, err
	}
	added := make([]WhitelistEntryView, 0, len(userIDs))
	for _, id := range userIDs {
		u := byID[id]
		entry := whitelist.Entry{
			UserID:     u.ID,
			FirstName:  u.FirstName,
			LastName:   u.LastName,
			Username:   u.Username,
			ApprovedBy: callerID,
			ApprovedAt: now,
			ExpiresAt:  now.Add(d.whitelistTTL),
			Note:       note,
			DeleteAt:   end,
		}
		if err := d.putWhitelistEntry(ctx, channelID, entry); err != nil {
			return nil, err
		}
		added = append(added, d.entryView(entry))
	}

	// Reflect it in the scan the admin is looking at.
	d.scansMutex.Lock()
	if job, ok := d.scans[scanID]; ok {
		for i := range job.view.Users {
			for _, e := range added {
				if job.view.Users[i].ID == e.UserID {
					markWhitelisted(&job.view.Users[i], e.Entry, now)
				}
			}
		}
	}
	d.scansMutex.Unlock()

	users := make([]audit.User, 0, len(added))
	names := make([]string, 0, len(added))
	for _, e := range added {
		users = append(users, auditUser(e.Entry))
		names = append(names, html.EscapeString(entryName(e.Entry)))
	}
	until := fmt.Sprintf("review on %s", formatDate(now.Add(d.whitelistTTL)))
	if end != nil && !end.After(now.Add(d.whitelistTTL)) {
		until = fmt.Sprintf("removed on %s", formatDate(*end))
	} else if end != nil {
		until += fmt.Sprintf(", removed on %s", formatDate(*end))
	}
	text := fmt.Sprintf("<b>Whitelist of %s</b>\n\n%s added by %s (%s).",
		html.EscapeString(d.channelTitle(channelID)), strings.Join(names, ", "),
		actorLink(callerID, d.userName(callerID)), until)
	if note != "" {
		text += "\nNote: " + html.EscapeString(note)
	}
	d.recordAudit(ctx, pc, audit.Event{
		ActorID: callerID, Action: audit.ActionWhitelistAdded, Users: users, Note: note,
		Details: whitelistDetails(now.Add(d.whitelistTTL), end),
	}, text)
	return added, nil
}

// RenewWhitelistEntry implements MiniAppService: a channel administrator
// reviews the user again and approves them for another period. A non-empty
// note replaces the previous one.
func (d *Domain) RenewWhitelistEntry(ctx context.Context, channelID, userID int64, note string, ttl time.Duration, callerID int64) (WhitelistEntryView, error) {
	if d.whitelistStorage == nil {
		return WhitelistEntryView{}, ErrWhitelistDisabled
	}
	note, err := cleanNote(note)
	if err != nil {
		return WhitelistEntryView{}, err
	}
	pc, ok := d.getProtectedChannel(channelID)
	if !ok {
		return WhitelistEntryView{}, fmt.Errorf("channel %d is not protected", channelID)
	}
	entry, found := d.whitelistEntry(ctx, channelID, userID)
	if !found {
		return WhitelistEntryView{}, ErrNotWhitelisted
	}
	now := d.now()
	end, err := d.deleteAt(ttl, now)
	if err != nil {
		return WhitelistEntryView{}, err
	}
	if end != nil {
		// A new term replaces the old end; without one the old end stays.
		entry.DeleteAt = end
	}
	entry.ApprovedBy = callerID
	entry.ApprovedAt = now
	entry.ExpiresAt = now.Add(d.whitelistTTL)
	entry.ExpiredRemindedAt = time.Time{}
	if note != "" {
		entry.Note = note
	}
	if err := d.putWhitelistEntry(ctx, channelID, entry); err != nil {
		return WhitelistEntryView{}, err
	}
	d.recordAudit(ctx, pc, audit.Event{
		ActorID: callerID, Action: audit.ActionWhitelistRenewed, Users: []audit.User{auditUser(entry)}, Note: note,
		Details: whitelistDetails(entry.ExpiresAt, entry.DeleteAt),
	}, fmt.Sprintf("<b>Whitelist of %s</b>\n\n%s re-approved by %s until %s.",
		html.EscapeString(d.channelTitle(channelID)), html.EscapeString(entryName(entry)),
		actorLink(callerID, d.userName(callerID)), formatDate(entry.ExpiresAt)))
	return d.entryView(entry), nil
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
	entry, found := d.whitelistEntry(ctx, channelID, userID)
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
	d.recordAudit(ctx, pc, audit.Event{
		ActorID: callerID, Action: audit.ActionWhitelistRemoved, Users: []audit.User{auditUser(entry)},
	}, fmt.Sprintf("<b>Whitelist of %s</b>\n\n%s removed by %s. They are checked as usual from the next scan.",
		html.EscapeString(d.channelTitle(channelID)), html.EscapeString(entryName(entry)),
		actorLink(callerID, d.userName(callerID))))
	return nil
}

// RunWhitelistReminders periodically reminds managers about approvals that
// are about to run out and, every day, about overdue ones.
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
		var expiring, overdue, ending, ended []whitelist.Entry
		for _, e := range entries {
			switch {
			case e.Deleted(now):
				ended = append(ended, e)
			case e.EndsBeforeReview():
				// A temporary approval that ends before its review: warn
				// about the end instead.
				if e.DeleteAt.Sub(now) <= d.whitelistRemindBefore && !e.RemindedFor.Equal(*e.DeleteAt) {
					ending = append(ending, e)
				}
			case e.Expired(now) && now.Sub(e.ExpiredRemindedAt) >= whitelistExpiredRemindEvery:
				overdue = append(overdue, e)
			case !e.Expired(now) && e.ExpiresAt.Sub(now) <= d.whitelistRemindBefore && !e.RemindedFor.Equal(e.ExpiresAt):
				expiring = append(expiring, e)
			}
		}
		d.whitelistMutex.Unlock()
		d.removeEndedEntries(ctx, pc, ended)
		if len(expiring) == 0 && len(overdue) == 0 && len(ending) == 0 {
			continue
		}
		sort.Slice(overdue, func(i, j int) bool { return overdue[i].ExpiresAt.Before(overdue[j].ExpiresAt) })
		sort.Slice(expiring, func(i, j int) bool { return expiring[i].ExpiresAt.Before(expiring[j].ExpiresAt) })

		var text strings.Builder
		fmt.Fprintf(&text, "<b>Whitelist of %s needs a review</b>\n", html.EscapeString(d.channelTitle(pc.ID)))
		if len(overdue) > 0 {
			text.WriteString("\nApproval expired. They are still protected, but please check each of them and renew or remove:\n")
			for _, e := range overdue {
				fmt.Fprintf(&text, "• %s — since %s\n", html.EscapeString(entryName(e)), formatDate(e.ExpiresAt))
			}
		}
		if len(expiring) > 0 {
			text.WriteString("\nApproval runs out soon:\n")
			for _, e := range expiring {
				fmt.Fprintf(&text, "• %s — until %s\n", html.EscapeString(entryName(e)), formatDate(e.ExpiresAt))
			}
		}
		if len(ending) > 0 {
			text.WriteString("\nTemporary approval ends soon; they will be checked as usual after:\n")
			for _, e := range ending {
				fmt.Fprintf(&text, "• %s — %s\n", html.EscapeString(entryName(e)), formatDate(*e.DeleteAt))
			}
		}
		if d.miniAppURL != "" {
			text.WriteString("\nOpen the app: /app → Whitelist.")
		}
		sentToChats := d.notifyControlChats(ctx, pc, text.String())
		d.notifyManagers(ctx, pc, text.String())
		if !sentToChats && len(pc.Managers) == 0 {
			// Nobody got it; retry on the next tick instead of marking it sent.
			continue
		}

		for _, e := range expiring {
			e.RemindedFor = e.ExpiresAt
			if err := d.putWhitelistEntry(ctx, pc.ID, e); err != nil {
				d.log.Errorf("%s", err)
			}
		}
		for _, e := range ending {
			e.RemindedFor = *e.DeleteAt
			if err := d.putWhitelistEntry(ctx, pc.ID, e); err != nil {
				d.log.Errorf("%s", err)
			}
		}
		var newlyExpired []audit.User
		for _, e := range overdue {
			if e.ExpiredRemindedAt.IsZero() {
				newlyExpired = append(newlyExpired, auditUser(e))
			}
			e.ExpiredRemindedAt = now
			if err := d.putWhitelistEntry(ctx, pc.ID, e); err != nil {
				d.log.Errorf("%s", err)
			}
		}
		if len(newlyExpired) > 0 {
			d.recordAudit(ctx, pc, audit.Event{Action: audit.ActionWhitelistExpired, Users: newlyExpired}, "")
		}
	}
}

// notifyControlChats sends text to the channel's control chats, with a
// button opening the channel in the Mini App when a direct link is possible.
// It reports whether at least one chat got the message.
func (d *Domain) notifyControlChats(ctx context.Context, pc ProtectedChannel, text string) bool {
	sent := false
	for _, chatID := range pc.CommandChannelIDs {
		if err := d.telegramBot.SendMessage(ctx, d.withAppButton(pc.ID, chatID, text)); err != nil {
			d.log.Errorf("can't notify control chat %d: %s", chatID, err)
			continue
		}
		sent = true
	}
	return sent
}

// withAppButton builds a message to chatID that opens channelID in the Mini
// App via a direct link (works in groups and private chats alike).
func (d *Domain) withAppButton(channelID, chatID int64, text string) *guard.Message {
	msg := &guard.Message{ChatID: chatID, Text: text}
	if d.miniAppShortName != "" && d.telegramBot.Username() != "" {
		msg.InlineButtons = [][]guard.InlineButton{{{
			Text: "Open NDA Guard",
			URL:  fmt.Sprintf("https://t.me/%s/%s?startapp=%d", d.telegramBot.Username(), d.miniAppShortName, channelID),
		}}}
	}
	return msg
}

// markWhitelisted flags a scanned user as covered by entry.
func markWhitelisted(u *ScannedUser, e whitelist.Entry, now time.Time) {
	u.Status = UserStatusWhitelisted
	if !u.Protected {
		u.Protected = true
		u.Note = noteWhitelisted
	}
	u.Whitelist = &ScannedWhitelist{Until: e.ExpiresAt, Expired: e.Expired(now), Note: e.Note, DeleteAt: e.DeleteAt}
}

func auditUser(e whitelist.Entry) audit.User {
	return audit.User{ID: e.UserID, Name: strings.TrimSpace(e.FirstName + " " + e.LastName), Username: e.Username}
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

// actorLink renders a user mention for HTML chat messages.
func actorLink(userID int64, name string) string {
	if name == "" {
		name = fmt.Sprintf("%d", userID)
	}
	return fmt.Sprintf("<a href=\"tg://user?id=%d\">%s</a>", userID, html.EscapeString(name))
}

func formatDate(t time.Time) string {
	return t.UTC().Format("02.01.2006")
}

// removeEndedEntries drops temporary approvals that reached their end.
func (d *Domain) removeEndedEntries(ctx context.Context, pc ProtectedChannel, ended []whitelist.Entry) {
	if len(ended) == 0 {
		return
	}
	users := make([]audit.User, 0, len(ended))
	names := make([]string, 0, len(ended))
	for _, e := range ended {
		storeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		err := d.whitelistStorage.DropWhitelistEntry(storeCtx, pc.ID, e.UserID)
		cancel()
		if err != nil {
			d.log.Errorf("can't drop ended whitelist entry %d of %d: %s", e.UserID, pc.ID, err)
			continue
		}
		d.whitelistMutex.Lock()
		delete(d.whitelists[pc.ID], e.UserID)
		d.whitelistMutex.Unlock()
		users = append(users, auditUser(e))
		names = append(names, html.EscapeString(entryName(e)))
	}
	if len(users) == 0 {
		return
	}
	d.recordAudit(ctx, pc, audit.Event{
		Action: audit.ActionWhitelistRemoved, Users: users,
		Details: map[string]any{"reason": "term_ended"},
	}, fmt.Sprintf("<b>Whitelist of %s</b>\n\nTemporary approval ended for %s. They are checked as usual from the next scan.",
		html.EscapeString(d.channelTitle(pc.ID)), strings.Join(names, ", ")))
}

func whitelistDetails(reviewAt time.Time, deleteAt *time.Time) map[string]any {
	details := map[string]any{"reviewAt": reviewAt}
	if deleteAt != nil {
		details["deleteAt"] = *deleteAt
	}
	return details
}
