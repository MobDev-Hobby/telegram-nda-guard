package scanner

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	guard "github.com/MobDev-Hobby/telegram-nda-guard"
	"github.com/MobDev-Hobby/telegram-nda-guard/processors"
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
}

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
	SetChannelSettings(ctx context.Context, channelID int64, settings ChannelSettings) error
	// StartChannelScan lists and classifies the channel's members in the
	// background. A scan already running for the channel is returned instead
	// of starting another.
	StartChannelScan(ctx context.Context, channelID, callerID int64) (ScanView, error)
	GetChannelScan(ctx context.Context, channelID int64, scanID string) (ScanView, error)
	// KickScannedUsers removes the selected users. Only users returned by the
	// given scan can be kicked, never protected ones.
	KickScannedUsers(ctx context.Context, channelID int64, scanID string, userIDs []int64, callerID int64) ([]processors.KickResult, error)
}

var _ MiniAppService = (*Domain)(nil)

type scanJob struct {
	view  ScanView
	users map[int64]guard.User
}

// SetChannelSettings implements MiniAppService.
func (d *Domain) SetChannelSettings(ctx context.Context, channelID int64, settings ChannelSettings) error {
	return d.updateProtectedChannel(ctx, channelID, func(pc *ProtectedChannel) {
		pc.AutoScan = settings.AutoScan
		pc.AutoClean = settings.AutoClean
		pc.AllowClean = settings.AllowClean
		pc.CleanOptions = &processors.CleanOptions{
			KeepBanned:    settings.KeepBanned,
			CleanMessages: settings.CleanMessages,
			CleanUnknown:  settings.CleanUnknown,
		}
	})
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

	// The scan outlives the HTTP request that started it.
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
		status := UserStatusGood
		hasAccess, err := checker.HasAccess(ctx, &user)
		switch {
		case err != nil:
			status = UserStatusUnknown
		case !hasAccess:
			status = UserStatusBad
		}
		note, isProtected := protected[user.ID]
		scanned = append(scanned, ScannedUser{
			ID:        user.ID,
			FirstName: user.FirstName,
			LastName:  user.LastName,
			Username:  user.Username,
			Status:    status,
			Protected: isProtected,
			Note:      note,
		})
		byID[user.ID] = user

		d.scansMutex.Lock()
		job.view.Checked = i + 1
		d.scansMutex.Unlock()
	}

	sort.SliceStable(scanned, func(i, j int) bool {
		return statusOrder(scanned[i].Status) < statusOrder(scanned[j].Status)
	})

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

	okCount := 0
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
	d.reportManualKick(ctx, pc, channel, callerID, okCount, len(toKick))
	return results, nil
}

// reportManualKick leaves an audit trail of Mini App kicks in the control
// chats, like the automatic clean reports do.
func (d *Domain) reportManualKick(ctx context.Context, pc ProtectedChannel, channel guard.ChannelInfo, callerID int64, kicked, selected int) {
	if selected == 0 {
		return
	}
	text := fmt.Sprintf(
		"<b>Manual clean of %s %s</b>\n\n"+
			"Removed <b>%d/%d</b> selected users.\n"+
			"Requested by <a href=\"tg://user?id=%d\">%d</a> via the Mini App.",
		guard.ChatTypeNoun(channel.Type),
		channel.Title,
		kicked,
		selected,
		callerID,
		callerID,
	)
	for _, chatID := range pc.CommandChannelIDs {
		if err := d.telegramBot.SendMessage(ctx, &guard.Message{ChatID: chatID, Text: text}); err != nil {
			d.log.Errorf("can't send manual clean report to %d: %s", chatID, err)
		}
	}
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
	case UserStatusKicked:
		return 3
	default:
		return 2
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
