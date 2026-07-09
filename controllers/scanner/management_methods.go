package scanner

import (
	"context"
	"fmt"
	"time"

	guard "github.com/MobDev-Hobby/telegram-nda-guard"
	"github.com/MobDev-Hobby/telegram-nda-guard/storage/channels"
)

// channelView builds a value-copy ChannelView from the in-memory caches. The
// caller must hold d.channelsMutex (read or write).
func (d *Domain) channelViewLocked(channelID int64) (ChannelView, bool) {
	ch, ok := d.channels[channelID]
	if !ok {
		// Channel is known to storage but its runtime cache entry hasn't been
		// populated yet (e.g. before the first CheckRights). Build a minimal
		// view from the protected-channel record if present.
		pc, pcOK := d.protectedChannels[channelID]
		if !pcOK {
			return ChannelView{}, false
		}
		return ChannelView{
			ID:           pc.ID,
			Title:        fmt.Sprintf("%d", channelID),
			CommandChats: append([]int64(nil), pc.CommandChannelIDs...),
			AutoScan:     pc.AutoScan,
			AutoClean:    pc.AutoClean,
			AllowClean:   pc.AllowClean,
		}, true
	}
	pc, _ := d.protectedChannels[channelID]
	return ChannelView{
		ID:           ch.id,
		Title:        ch.title,
		ChatType:     ch.chatType,
		CommandChats: append([]int64(nil), ch.commandChannelIDs...),
		AutoScan:     pc.AutoScan,
		AutoClean:    pc.AutoClean,
		AllowClean:   pc.AllowClean,
		BotOnChannel: ch.botOnChannel,
		BotCanInvite: ch.botCanInvite,
		BotCanClean:  ch.botCanClean,
	}, true
}

// ListChannels implements ManagementService.
func (d *Domain) ListChannels(_ context.Context, commandChatID int64) ([]ChannelView, error) {
	d.channelsMutex.RLock()
	defer d.channelsMutex.RUnlock()

	if commandChatID == 0 {
		out := make([]ChannelView, 0, len(d.protectedChannels))
		for id := range d.protectedChannels {
			if v, ok := d.channelViewLocked(id); ok {
				out = append(out, v)
			}
		}
		return out, nil
	}

	ids := d.commandChannels[commandChatID]
	out := make([]ChannelView, 0, len(ids))
	for _, id := range ids {
		if v, ok := d.channelViewLocked(id); ok {
			out = append(out, v)
		}
	}
	return out, nil
}

// GetChannel implements ManagementService.
func (d *Domain) GetChannel(_ context.Context, channelID int64) (ChannelView, error) {
	d.channelsMutex.RLock()
	defer d.channelsMutex.RUnlock()
	v, ok := d.channelViewLocked(channelID)
	if !ok {
		return ChannelView{}, fmt.Errorf("channel %d not found", channelID)
	}
	return v, nil
}

// AddChannel implements ManagementService. It wires the channel to the default
// processors/checker (the same path the /add command uses) and then re-resolves
// the bot's rights so the returned view reflects reality.
func (d *Domain) AddChannel(ctx context.Context, channelID, commandChatID int64, autoScan, autoClean, allowClean bool) error {
	if d.defaultAccessChecker == nil || d.defaultCleanProcessor == nil || d.defaultScanProcessor == nil {
		return fmt.Errorf("default access checker, clean processor and scan processor must be configured")
	}
	pc := &ProtectedChannel{
		ID:                channelID,
		CommandChannelIDs: []int64{commandChatID},
		AutoScan:          autoScan,
		AutoClean:         autoClean,
		AllowClean:        allowClean,
	}
	if err := d.AddDefaultProtectedChannel(pc); err != nil {
		return fmt.Errorf("add protected channel: %w", err)
	}
	if err := d.CheckRights(ctx); err != nil {
		// Non-fatal: the channel is added; rights just couldn't be refreshed.
		d.log.Errorf("AddChannel: can't check rights: %v", err)
	}
	return nil
}

// RemoveChannel implements ManagementService. It refuses removal when
// commandChatID does not control channelID.
func (d *Domain) RemoveChannel(ctx context.Context, channelID, commandChatID int64) error {
	d.channelsMutex.RLock()
	controlled := false
	for _, id := range d.commandChannels[commandChatID] {
		if id == channelID {
			controlled = true
			break
		}
	}
	d.channelsMutex.RUnlock()
	if !controlled {
		return fmt.Errorf("channel %d is not controlled from chat %d", channelID, commandChatID)
	}
	return d.CleanProtectedChannel(channelID, commandChatID)
}

// SetChannelFlags implements ManagementService. It sets the three flags to the
// given absolute values (unlike the /setflag toggle), persists and syncs the
// ticker. It reuses the persistence + ticker-sync path of applyFlagToggle.
func (d *Domain) SetChannelFlags(ctx context.Context, channelID int64, autoScan, autoClean, allowClean bool) error {
	return d.applyFlagSet(ctx, channelID, autoScan, autoClean, allowClean)
}

// applyFlagSet is the absolute-value counterpart of applyFlagToggle. It is kept
// separate so the toggle handler's semantics (flip one flag) stay untouched.
func (d *Domain) applyFlagSet(ctx context.Context, channelID int64, autoScan, autoClean, allowClean bool) error {
	d.channelsMutex.Lock()
	pc, ok := d.protectedChannels[channelID]
	if !ok {
		d.channelsMutex.Unlock()
		return fmt.Errorf("channel %d not found", channelID)
	}
	pc.AutoScan = autoScan
	pc.AutoClean = autoClean
	pc.AllowClean = allowClean

	if d.storage != nil {
		storeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		if err := d.storage.Store(storeCtx, &channels.ProtectedChannel{
			ID:                pc.ID,
			CommandChannelIDs: pc.CommandChannelIDs,
			AutoScan:          pc.AutoScan,
			AutoClean:         pc.AutoClean,
			AllowClean:        pc.AllowClean,
		}); err != nil {
			d.channelsMutex.Unlock()
			return fmt.Errorf("persist channel: %w", err)
		}
	}
	d.protectedChannels[channelID] = pc
	needsTicker := pc.AutoScan || pc.AutoClean
	d.channelsMutex.Unlock()

	hasTicker := d.channelHasTicker(channelID)
	switch {
	case needsTicker && !hasTicker:
		ticker := time.NewTicker(d.channelAutoScanInterval)
		_ = d.registerTicker(ticker.C, channelID)
	case !needsTicker && hasTicker:
		d.removeTickers(channelID)
	}
	d.log.Infof("set flags for channel %d -> autoscan=%t autoclean=%t allowclean=%t",
		channelID, pc.AutoScan, pc.AutoClean, pc.AllowClean)
	return nil
}

// ListChannelUsers implements ManagementService. It fetches members via the
// userbot and classifies them with the channel's access checker (or the default
// checker). This is the structured counterpart of the /users command.
func (d *Domain) ListChannelUsers(ctx context.Context, channelID int64) (UsersView, error) {
	d.channelsMutex.RLock()
	pc, ok := d.protectedChannels[channelID]
	ch := d.channels[channelID]
	d.channelsMutex.RUnlock()
	if !ok {
		return UsersView{}, fmt.Errorf("channel %d is not protected", channelID)
	}
	if d.userBot == nil {
		return UsersView{}, fmt.Errorf("userbot is not running")
	}
	users, err := d.userBot.GetChannelUsers(ctx, channelID)
	if err != nil {
		return UsersView{}, fmt.Errorf("get channel users: %w", err)
	}
	checker := pc.AccessChecker
	if checker == nil {
		checker = d.defaultAccessChecker
	}
	if checker == nil {
		return UsersView{}, fmt.Errorf("no access checker configured")
	}

	view := UsersView{
		ChannelID: channelID,
		Title:     ch.title,
		Good:      []guard.User{},
		Unknown:   []guard.User{},
		Bad:       []guard.User{},
	}
	for _, user := range users {
		user := user
		hasAccess, err := checker.HasAccess(ctx, &user)
		if err != nil {
			view.Unknown = append(view.Unknown, user)
			continue
		}
		if hasAccess {
			view.Good = append(view.Good, user)
		} else {
			view.Bad = append(view.Bad, user)
		}
	}
	return view, nil
}

// TriggerScan implements ManagementService. It enqueues a manual scan; the
// report is sent to reportChannelID.
func (d *Domain) TriggerScan(ctx context.Context, channelID, reportChannelID int64) error {
	return d.enqueueScan(ctx, channelID, reportChannelID, Scan)
}

// TriggerClean implements ManagementService.
func (d *Domain) TriggerClean(ctx context.Context, channelID, reportChannelID int64) error {
	return d.enqueueScan(ctx, channelID, reportChannelID, Clean)
}

// enqueueScan builds a ScanRequest from the in-memory caches and submits it to
// the worker pool, mirroring the /scan and /clean command path.
func (d *Domain) enqueueScan(_ context.Context, channelID, reportChannelID int64, requestType ScanRequestType) error {
	d.channelsMutex.RLock()
	ch, ok := d.channels[channelID]
	pc, pcOK := d.protectedChannels[channelID]
	d.channelsMutex.RUnlock()
	if !ok || !pcOK {
		return fmt.Errorf("channel %d not found", channelID)
	}

	var reportProcessor UserReportProcessor
	switch requestType {
	case Scan:
		reportProcessor = pc.ScanReportProcessor
	case Clean:
		reportProcessor = pc.CleanReportProcessor
	default:
		return fmt.Errorf("unsupported request type %d for manual trigger", requestType)
	}
	if reportProcessor == nil {
		return fmt.Errorf("no report processor configured for request type %d", requestType)
	}

	chans := []int64{reportChannelID}
	d.processRequestChan <- ScanRequest{
		requestType:     requestType,
		channelInfo:     ch,
		accessChecker:   pc.AccessChecker,
		reportProcessor: reportProcessor,
		reportChannels:  &chans,
	}
	return nil
}

// GetStatus implements ManagementService.
func (d *Domain) GetStatus(ctx context.Context) (StatusView, error) {
	channels, err := d.ListChannels(ctx, 0)
	if err != nil {
		return StatusView{}, err
	}
	status := StatusView{Channels: channels}
	if d.telegramBot != nil {
		status.BotUsername = d.telegramBot.Username()
		status.BotUserID = d.telegramBot.UserID()
	}
	if d.userBot != nil {
		status.UserBotUsername = d.userBot.Username()
		status.UserBotUserID = d.userBot.UserID()
	}
	status.AdminChatID = d.adminUserChatID
	return status, nil
}

// RefreshRights implements ManagementService.
func (d *Domain) RefreshRights(ctx context.Context) error {
	return d.CheckRights(ctx)
}

// Compile-time assertion: *Domain satisfies ManagementService.
var _ ManagementService = (*Domain)(nil)
