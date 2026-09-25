package scanner

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"html"
	"sort"
	"strings"
	"time"

	guard "github.com/MobDev-Hobby/telegram-nda-guard"
	"github.com/MobDev-Hobby/telegram-nda-guard/processors"
	"github.com/MobDev-Hobby/telegram-nda-guard/storage/audit"
	"github.com/MobDev-Hobby/telegram-nda-guard/storage/knownchats"
)

const (
	// scanJobTTL is how long a finished scan can still be used to kick.
	// Membership drifts; a stale scan must not drive kicks.
	scanJobTTL = 30 * time.Minute
	// scanJobTimeout bounds one scan: listing members plus checking each.
	scanJobTimeout = 10 * time.Minute
	// maxScanJobs caps the in-memory scan store.
	maxScanJobs = 200
	// maxKickBatch caps one kick request.
	maxKickBatch = 1000
)

// Scan user statuses.
const (
	UserStatusBad     = "bad"
	UserStatusUnknown = "unknown"
	UserStatusGood    = "good"
	UserStatusKicked  = "kicked"
	// UserStatusWhitelisted: the user is on the channel's whitelist and is not
	// checked or removed (even when the approval is overdue for review).
	UserStatusWhitelisted = "whitelisted"
)

// Scan states.
const (
	ScanStateRunning = "running"
	ScanStateDone    = "done"
	ScanStateFailed  = "failed"
)

var (
	ErrScanNotFound         = errors.New("scan not found or expired")
	ErrManualCleanDisabled  = errors.New("manual clean is disabled for this channel")
	ErrBotCannotClean       = errors.New("the bot has no right to ban users in this channel")
	ErrKickerNotConfigured  = errors.New("manual kicks are not configured")
	ErrScanNotFinished      = errors.New("scan is not finished yet")
	ErrTooManyUsersSelected = fmt.Errorf("at most %d users can be kicked at once", maxKickBatch)
)

// UserKicker removes selected users from a channel. processors/kicker.Domain
// implements it.
type UserKicker interface {
	KickUsers(
		ctx context.Context,
		channel guard.ChannelInfo,
		users []guard.User,
		opts *processors.CleanOptions,
	) []processors.KickResult
}

// ChannelSettings is every per-channel setting an administrator can change.
type ChannelSettings struct {
	AutoScan      bool `json:"autoScan"`
	AutoClean     bool `json:"autoClean"`
	AllowClean    bool `json:"allowClean"`
	KeepBanned    bool `json:"keepBanned"`
	CleanMessages bool `json:"cleanMessages"`
	CleanUnknown  bool `json:"cleanUnknown"`
	// JoinRequests is the join request mode: off, auto or manual.
	JoinRequests string `json:"joinRequests"`
}

// ScannedUser is one member in a scan result. Phone numbers are deliberately
// left out: the Mini App shows scan results to channel admins, who have no
// need for members' phones.
type ScannedUser struct {
	ID        int64  `json:"id"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName,omitempty"`
	Username  string `json:"username,omitempty"`
	Status    string `json:"status"`
	// Protected users (channel admins, the bot itself) can't be kicked.
	Protected bool   `json:"protected,omitempty"`
	Note      string `json:"note,omitempty"`
	// Whitelist is set for users on the channel's whitelist.
	Whitelist *ScannedWhitelist `json:"whitelist,omitempty"`
	// Check is the access checker's own verdict (good/bad/unknown) from the
	// latest explicit recheck; for whitelisted users it tells whether they
	// would pass without the whitelist.
	Check     string     `json:"check,omitempty"`
	CheckedAt *time.Time `json:"checkedAt,omitempty"`
}

// ScannedWhitelist is a scanned user's whitelist entry.
type ScannedWhitelist struct {
	Until    time.Time  `json:"until"`
	Expired  bool       `json:"expired"`
	Note     string     `json:"note,omitempty"`
	DeleteAt *time.Time `json:"deleteAt,omitempty"`
}

// noteWhitelisted marks users protected by an active whitelist approval.
const noteWhitelisted = "whitelisted"

// ScanView is the state of a scan started from the Mini App.
type ScanView struct {
	ID        string `json:"id"`
	ChannelID int64  `json:"channelId"`
	Title     string `json:"title"`
	ChatType  string `json:"chatType"`
	State     string `json:"state"`
	Error     string `json:"error,omitempty"`
	// Checked counts members checked so far; Stats.Fetched is known once the
	// member list has been fetched, so the two give the scan's progress.
	Checked    int             `json:"checked"`
	Stats      guard.ScanStats `json:"stats"`
	Partial    bool            `json:"partial"`
	Users      []ScannedUser   `json:"users,omitempty"`
	StartedAt  time.Time       `json:"startedAt"`
	FinishedAt *time.Time      `json:"finishedAt,omitempty"`
	StartedBy  int64           `json:"startedBy"`
}

// MiniAppService is the management surface of the Telegram Mini App. It is
// separate from ManagementService so existing implementations of that
// interface keep compiling. *Domain implements both.
type MiniAppService interface {
	ListChannels(ctx context.Context, commandChatID int64) ([]ChannelView, error)
	GetChannel(ctx context.Context, channelID int64) (ChannelView, error)
	// SetChannelSettings replaces every per-channel setting at once.
	SetChannelSettings(ctx context.Context, channelID int64, settings ChannelSettings, callerID int64) error
	// StartChannelScan lists and classifies the channel's members in the
	// background. A scan already running for the channel is returned instead
	// of starting another.
	StartChannelScan(ctx context.Context, channelID, callerID int64) (ScanView, error)
	GetChannelScan(ctx context.Context, channelID int64, scanID string) (ScanView, error)
	// KickScannedUsers removes the selected users. Only users returned by the
	// given scan can be kicked, never protected ones.
	KickScannedUsers(ctx context.Context, channelID int64, scanID string, userIDs []int64, callerID int64) ([]processors.KickResult, error)

	// RecheckScannedUser runs the access checker again for one scanned user,
	// bypassing its cache and the whitelist.
	RecheckScannedUser(ctx context.Context, channelID int64, scanID string, userID, callerID int64) (ScannedUser, error)

	// ListWhitelist returns the channel's whitelist, expired entries included.
	ListWhitelist(ctx context.Context, channelID int64) ([]WhitelistEntryView, error)
	// AddToWhitelist approves users from a scan of the channel for the
	// whitelist period, with the approver's note.
	// ttl > 0 makes the approval temporary: the entry is removed after ttl.
	AddToWhitelist(ctx context.Context, channelID int64, scanID string, userIDs []int64, note string, ttl time.Duration, callerID int64) ([]WhitelistEntryView, error)
	// RenewWhitelistEntry re-approves a user for another period; a non-empty
	// note replaces the previous one and ttl > 0 sets a new end.
	RenewWhitelistEntry(ctx context.Context, channelID, userID int64, note string, ttl time.Duration, callerID int64) (WhitelistEntryView, error)
	RemoveWhitelistEntry(ctx context.Context, channelID, userID, callerID int64) error

	// CanUseMiniApp tells whether user may use the Mini App at all: only
	// people who pass the access checker (employees) may.
	CanUseMiniApp(ctx context.Context, user guard.User) (bool, error)
	// NoteUserName remembers a display name for logs and messages.
	NoteUserName(userID int64, name string)
	// IsChannelManager tells whether userID joined the channel in the bot.
	IsChannelManager(channelID, userID int64) bool
	// JoinChannel makes a channel administrator a manager of it in the bot.
	JoinChannel(ctx context.Context, channelID, callerID int64) error
	// ListAudit returns the channel's action log, newest first.
	ListAudit(ctx context.Context, channelID int64, limit int) ([]audit.Event, error)

	// ListAvailableChats returns chats where the bot is an administrator but
	// which are not protected yet.
	ListAvailableChats(ctx context.Context) ([]knownchats.Chat, error)
	// ConnectChat protects such a chat, with the caller as manager.
	ConnectChat(ctx context.Context, chatID, callerID int64) error
	// BotUsername is used to open the bot's private chat from the app.
	BotUsername() string

	// ListJoinRequests returns pending requests to join the channel.
	ListJoinRequests(ctx context.Context, channelID int64) ([]JoinRequestView, error)
	// ResolveJoinRequests approves or declines pending requests.
	ResolveJoinRequests(ctx context.Context, channelID int64, userIDs []int64, approve bool, callerID int64) ([]JoinResult, error)
	// RecheckJoinRequest runs the checker again for one requester.
	RecheckJoinRequest(ctx context.Context, channelID, userID, callerID int64) (JoinRequestView, error)
}

// BotUsername implements MiniAppService.
func (d *Domain) BotUsername() string {
	return d.telegramBot.Username()
}

var _ MiniAppService = (*Domain)(nil)

type scanJob struct {
	view  ScanView
	users map[int64]guard.User
}

// SetChannelSettings implements MiniAppService.
func (d *Domain) SetChannelSettings(ctx context.Context, channelID int64, settings ChannelSettings, callerID int64) error {
	if !validJoinMode(settings.JoinRequests) {
		return ErrBadJoinMode
	}
	if settings.JoinRequests != "" && settings.JoinRequests != JoinModeOff && d.joinRequestStorage == nil {
		return ErrJoinRequestsDisabled
	}
	err := d.updateProtectedChannel(ctx, channelID, func(pc *ProtectedChannel) {
		pc.JoinRequestMode = settings.JoinRequests
		pc.AutoScan = settings.AutoScan
		pc.AutoClean = settings.AutoClean
		pc.AllowClean = settings.AllowClean
		pc.CleanOptions = &processors.CleanOptions{
			KeepBanned:    settings.KeepBanned,
			CleanMessages: settings.CleanMessages,
			CleanUnknown:  settings.CleanUnknown,
		}
	})
	if err != nil {
		return err
	}
	pc, _ := d.getProtectedChannel(channelID)
	d.recordAudit(ctx, pc, audit.Event{
		ActorID: callerID, Action: audit.ActionSettingsChanged,
		Details: map[string]any{
			"autoScan": settings.AutoScan, "autoClean": settings.AutoClean, "allowClean": settings.AllowClean,
			"keepBanned": settings.KeepBanned, "cleanMessages": settings.CleanMessages, "cleanUnknown": settings.CleanUnknown,
			"joinRequests": settings.JoinRequests,
		},
	}, fmt.Sprintf("Settings of %s changed by %s.", html.EscapeString(d.channelTitle(channelID)), actorLink(callerID, d.userName(callerID))))
	return nil
}

// StartChannelScan implements MiniAppService.
func (d *Domain) StartChannelScan(_ context.Context, channelID, callerID int64) (ScanView, error) {
	d.channelsMutex.RLock()
	pc, ok := d.protectedChannels[channelID]
	ch := d.channels[channelID]
	d.channelsMutex.RUnlock()
	if !ok {
		return ScanView{}, fmt.Errorf("channel %d is not protected", channelID)
	}
	if d.userBot == nil {
		return ScanView{}, errors.New("userbot is not running")
	}
	if !ch.CanScan() {
		return ScanView{}, fmt.Errorf("the bot is not an administrator of %s", guard.ChatTypeNoun(ch.chatType))
	}
	checker := pc.AccessChecker
	if checker == nil {
		checker = d.defaultAccessChecker
	}
	if checker == nil {
		return ScanView{}, errors.New("no access checker configured")
	}

	d.scansMutex.Lock()
	d.expireScansLocked()
	for _, job := range d.scans {
		if job.view.ChannelID == channelID && job.view.State == ScanStateRunning {
			view := job.view
			d.scansMutex.Unlock()
			return view, nil
		}
	}
	job := &scanJob{view: ScanView{
		ID:        newScanID(),
		ChannelID: channelID,
		Title:     ch.title,
		ChatType:  ch.chatType,
		State:     ScanStateRunning,
		StartedAt: time.Now(),
		StartedBy: callerID,
	}}
	d.scans[job.view.ID] = job
	view := job.view
	d.scansMutex.Unlock()

	// The scan outlives the HTTP request that started it. Whitelisted users
	// are marked separately, so the scan uses the plain checker and skips
	// them.
	go d.runScanJob(job, checker)
	return view, nil
}

func (d *Domain) runScanJob(job *scanJob, checker CheckUserAccess) {
	ctx, cancel := context.WithTimeout(context.Background(), scanJobTimeout)
	defer cancel()
	channelID := job.view.ChannelID

	fail := func(err error) {
		d.log.Errorf("mini app scan of %d failed: %s", channelID, err)
		d.scansMutex.Lock()
		now := time.Now()
		job.view.State = ScanStateFailed
		job.view.Error = err.Error()
		job.view.FinishedAt = &now
		d.scansMutex.Unlock()
	}

	users, err := d.userBot.GetChannelUsers(ctx, channelID)
	if err != nil {
		fail(fmt.Errorf("can't list members: %w", err))
		return
	}
	stats := d.channelStats(channelID, len(users))
	d.scansMutex.Lock()
	job.view.Stats = stats
	d.scansMutex.Unlock()

	protected := map[int64]string{
		d.telegramBot.UserID(): "this bot",
	}
	if d.userBot.UserID() != 0 {
		protected[d.userBot.UserID()] = "this bot"
	}
	admins, err := d.telegramBot.GetChatAdministrators(ctx, channelID)
	if err != nil {
		// Without the admin list we can't tell who must not be kicked.
		fail(fmt.Errorf("can't list administrators: %w", err))
		return
	}
	for _, id := range admins {
		if _, ok := protected[id]; !ok {
			protected[id] = "administrator"
		}
	}

	scanned := make([]ScannedUser, 0, len(users))
	byID := make(map[int64]guard.User, len(users))
	for i, user := range users {
		if ctx.Err() != nil {
			fail(ctx.Err())
			return
		}
		user := user
		note, isProtected := protected[user.ID]
		su := ScannedUser{
			ID:        user.ID,
			FirstName: user.FirstName,
			LastName:  user.LastName,
			Username:  user.Username,
			Protected: isProtected,
			Note:      note,
		}
		if entry, ok := d.whitelistEntry(ctx, channelID, user.ID); ok {
			// Whitelisted users are not checked and can't be ticked for
			// removal; to remove one, take them off the whitelist first.
			markWhitelisted(&su, entry, d.now())
		} else {
			su.Status = checkStatus(checker.HasAccess(ctx, &user))
		}
		scanned = append(scanned, su)
		byID[user.ID] = user

		d.scansMutex.Lock()
		job.view.Checked = i + 1
		d.scansMutex.Unlock()
	}

	sort.SliceStable(scanned, func(i, j int) bool {
		return statusOrder(scanned[i].Status) < statusOrder(scanned[j].Status)
	})

	// Record the outcome before publishing "done", so a client that sees the
	// finished scan also sees the channel's updated health.
	summary := summarizeScan(ScanView{Users: scanned, Partial: stats.Partial()}, d.now())
	d.recordCheck(ctx, channelID, summary)
	if pc, ok := d.getProtectedChannel(channelID); ok {
		d.recordAudit(ctx, pc, audit.Event{
			ActorID: job.view.StartedBy, Action: audit.ActionScanCompleted,
			Counts:  summaryCounts(summary),
			Details: map[string]any{"source": "miniapp", "partial": summary.Partial},
		}, "")
	}

	d.scansMutex.Lock()
	now := time.Now()
	job.users = byID
	job.view.Users = scanned
	job.view.Stats = stats
	job.view.Partial = stats.Partial()
	job.view.State = ScanStateDone
	job.view.FinishedAt = &now
	d.scansMutex.Unlock()
}

// checkStatus maps a checker verdict to a scan status.
func checkStatus(hasAccess bool, err error) string {
	switch {
	case err != nil:
		return UserStatusUnknown
	case !hasAccess:
		return UserStatusBad
	default:
		return UserStatusGood
	}
}

// summarizeScan counts a scan's current statuses for the health indicator.
// Kicked users no longer count as violations. Caller holds scansMutex.
func summarizeScan(view ScanView, now time.Time) processors.CheckSummary {
	s := processors.CheckSummary{At: now, Partial: view.Partial}
	for _, u := range view.Users {
		switch u.Status {
		case UserStatusGood:
			s.Good++
		case UserStatusBad:
			s.Bad++
		case UserStatusUnknown:
			s.Unknown++
		case UserStatusWhitelisted:
			s.Whitelisted++
		}
	}
	return s
}

func summaryCounts(s processors.CheckSummary) map[string]int {
	return map[string]int{"good": s.Good, "bad": s.Bad, "unknown": s.Unknown, "whitelisted": s.Whitelisted}
}

// RecheckScannedUser implements MiniAppService.
func (d *Domain) RecheckScannedUser(ctx context.Context, channelID int64, scanID string, userID, callerID int64) (ScannedUser, error) {
	pc, ok := d.getProtectedChannel(channelID)
	if !ok {
		return ScannedUser{}, fmt.Errorf("channel %d is not protected", channelID)
	}
	checker := pc.AccessChecker
	if checker == nil {
		checker = d.defaultAccessChecker
	}
	if checker == nil {
		return ScannedUser{}, errors.New("no access checker configured")
	}

	d.scansMutex.Lock()
	job, ok := d.scans[scanID]
	if !ok || job.view.ChannelID != channelID || job.view.State != ScanStateDone {
		d.scansMutex.Unlock()
		return ScannedUser{}, ErrScanNotFound
	}
	user, ok := job.users[userID]
	d.scansMutex.Unlock()
	if !ok {
		return ScannedUser{}, fmt.Errorf("user %d is not in this scan", userID)
	}

	// A recheck must ask the source again, not the cached verdict.
	if invalidator, ok := checker.(interface{ Invalidate(userID int64) }); ok {
		invalidator.Invalidate(userID)
	}
	verdict := checkStatus(checker.HasAccess(ctx, &user))
	now := d.now()

	d.scansMutex.Lock()
	var updated ScannedUser
	for i := range job.view.Users {
		u := &job.view.Users[i]
		if u.ID != userID {
			continue
		}
		u.Check = verdict
		u.CheckedAt = &now
		if u.Status != UserStatusWhitelisted && u.Status != UserStatusKicked {
			u.Status = verdict
		}
		updated = *u
	}
	summary := summarizeScan(job.view, now)
	d.scansMutex.Unlock()

	d.recordCheck(ctx, channelID, summary)
	d.recordAudit(ctx, pc, audit.Event{
		ActorID: callerID, Action: audit.ActionUserRechecked,
		Users:   []audit.User{{ID: user.ID, Name: strings.TrimSpace(user.FirstName + " " + user.LastName), Username: user.Username}},
		Details: map[string]any{"result": verdict, "whitelisted": updated.Status == UserStatusWhitelisted},
	}, "")
	return updated, nil
}

// CanUseMiniApp implements MiniAppService. It uses the default access checker
// (the one that decides who may stay in channels), so only employees get in.
// A failed check denies access.
func (d *Domain) CanUseMiniApp(ctx context.Context, user guard.User) (bool, error) {
	if d.defaultAccessChecker == nil {
		return false, errors.New("no access checker configured")
	}
	d.NoteUserName(user.ID, strings.TrimSpace(user.FirstName+" "+user.LastName))
	return d.defaultAccessChecker.HasAccess(ctx, &user)
}

// GetChannelScan implements MiniAppService.
func (d *Domain) GetChannelScan(_ context.Context, channelID int64, scanID string) (ScanView, error) {
	d.scansMutex.Lock()
	defer d.scansMutex.Unlock()
	d.expireScansLocked()
	job, ok := d.scans[scanID]
	if !ok || job.view.ChannelID != channelID {
		return ScanView{}, ErrScanNotFound
	}
	view := job.view
	view.Users = append([]ScannedUser(nil), job.view.Users...)
	return view, nil
}

// KickScannedUsers implements MiniAppService.
func (d *Domain) KickScannedUsers(
	ctx context.Context,
	channelID int64,
	scanID string,
	userIDs []int64,
	callerID int64,
) ([]processors.KickResult, error) {
	if d.userKicker == nil {
		return nil, ErrKickerNotConfigured
	}
	if len(userIDs) > maxKickBatch {
		return nil, ErrTooManyUsersSelected
	}

	d.channelsMutex.RLock()
	pc, ok := d.protectedChannels[channelID]
	ch := d.channels[channelID]
	d.channelsMutex.RUnlock()
	if !ok {
		return nil, fmt.Errorf("channel %d is not protected", channelID)
	}
	if !pc.AllowClean {
		return nil, ErrManualCleanDisabled
	}
	if !ch.CanClean() {
		return nil, ErrBotCannotClean
	}

	// Resolve the selection against the scan under the lock, then kick
	// without holding it: kicks are rate limited and can take minutes.
	results := make([]processors.KickResult, 0, len(userIDs))
	toKick := make([]guard.User, 0, len(userIDs))
	d.scansMutex.Lock()
	d.expireScansLocked()
	job, ok := d.scans[scanID]
	if !ok || job.view.ChannelID != channelID {
		d.scansMutex.Unlock()
		return nil, ErrScanNotFound
	}
	if job.view.State != ScanStateDone {
		d.scansMutex.Unlock()
		return nil, ErrScanNotFinished
	}
	statusByID := make(map[int64]ScannedUser, len(job.view.Users))
	for _, u := range job.view.Users {
		statusByID[u.ID] = u
	}
	seen := make(map[int64]bool, len(userIDs))
	for _, id := range userIDs {
		if seen[id] {
			continue
		}
		seen[id] = true
		scanned, found := statusByID[id]
		switch {
		case !found:
			results = append(results, processors.KickResult{UserID: id, Error: "not found in this scan"})
		case scanned.Protected:
			results = append(results, processors.KickResult{UserID: id, Error: "protected: " + scanned.Note})
		case scanned.Status == UserStatusKicked:
			results = append(results, processors.KickResult{UserID: id, Error: "already kicked"})
		default:
			toKick = append(toKick, job.users[id])
		}
	}
	d.scansMutex.Unlock()

	channel := guard.ChannelInfo{ID: channelID, Title: ch.title, Type: ch.chatType}
	kicked := d.userKicker.KickUsers(ctx, channel, toKick, pc.CleanOptions)
	results = append(results, kicked...)

	d.scansMutex.Lock()
	if job, ok := d.scans[scanID]; ok {
		kickedIDs := make(map[int64]bool, len(kicked))
		for _, r := range kicked {
			if r.OK {
				kickedIDs[r.UserID] = true
			}
		}
		for i := range job.view.Users {
			if kickedIDs[job.view.Users[i].ID] {
				job.view.Users[i].Status = UserStatusKicked
			}
		}
	}
	d.scansMutex.Unlock()
	okCount := 0
	for _, r := range kicked {
		if r.OK {
			okCount++
		}
	}

	if okCount > 0 {
		if invalidator, ok := d.userBot.(interface{ InvalidateChannel(channelID int64) }); ok {
			invalidator.InvalidateChannel(channelID)
		}
	}
	d.scansMutex.Lock()
	var summary processors.CheckSummary
	if job, ok := d.scans[scanID]; ok {
		summary = summarizeScan(job.view, d.now())
	}
	d.scansMutex.Unlock()
	d.recordCheck(ctx, channelID, summary)
	d.reportManualKick(ctx, pc, channel, callerID, kicked, toKick)
	return results, nil
}

// reportManualKick leaves an audit trail of Mini App kicks in the log and in
// the control chats, like the automatic clean reports do.
func (d *Domain) reportManualKick(ctx context.Context, pc ProtectedChannel, channel guard.ChannelInfo, callerID int64, results []processors.KickResult, selected []guard.User) {
	if len(selected) == 0 {
		return
	}
	okIDs := make(map[int64]bool, len(results))
	for _, r := range results {
		if r.OK {
			okIDs[r.UserID] = true
		}
	}
	users := make([]audit.User, 0, len(okIDs))
	for _, u := range selected {
		if okIDs[u.ID] {
			users = append(users, audit.User{ID: u.ID, Name: strings.TrimSpace(u.FirstName + " " + u.LastName), Username: u.Username})
		}
	}
	text := fmt.Sprintf(
		"<b>Manual clean of %s %s</b>\n\n"+
			"Removed <b>%d/%d</b> selected users.\n"+
			"Requested by %s via the Mini App.",
		guard.ChatTypeNoun(channel.Type),
		html.EscapeString(channel.Title),
		len(users),
		len(selected),
		actorLink(callerID, d.userName(callerID)),
	)
	d.recordAudit(ctx, pc, audit.Event{
		ActorID: callerID, Action: audit.ActionUsersKicked, Users: users,
		Counts: map[string]int{"kicked": len(users), "selected": len(selected)},
	}, text)
}

// expireScansLocked drops finished scans past their TTL and, when the store is
// full, the oldest ones. Caller holds scansMutex.
func (d *Domain) expireScansLocked() {
	now := time.Now()
	for id, job := range d.scans {
		if job.view.FinishedAt != nil && now.Sub(*job.view.FinishedAt) > scanJobTTL {
			delete(d.scans, id)
		}
	}
	for len(d.scans) >= maxScanJobs {
		var oldestID string
		var oldest time.Time
		for id, job := range d.scans {
			if oldestID == "" || job.view.StartedAt.Before(oldest) {
				oldestID, oldest = id, job.view.StartedAt
			}
		}
		delete(d.scans, oldestID)
	}
}

func statusOrder(status string) int {
	switch status {
	case UserStatusBad:
		return 0
	case UserStatusUnknown:
		return 1
	case UserStatusWhitelisted:
		return 2
	case UserStatusKicked:
		return 4
	default:
		return 3
	}
}

func newScanID() string {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		// crypto/rand does not fail on supported platforms.
		panic(err)
	}
	return strings.ToLower(hex.EncodeToString(buf))
}
