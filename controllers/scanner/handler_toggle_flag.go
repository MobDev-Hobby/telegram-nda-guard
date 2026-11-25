package scanner

import (
	"context"
	"fmt"
	"strings"
	"time"

	guard "github.com/MobDev-Hobby/telegram-nda-guard"
)

// ToggleFlagHandler flips one of the AutoScan/AutoClean/AllowClean flags for a
// channel, persists the change, and keeps the scan/clean ticker in sync. It is
// triggered by the /setflag <id> <flag> inline button.
func (d *Domain) ToggleFlagHandler(
	ctx context.Context,
	update *guard.Update,
) {
	if update.CallbackQuery == nil || update.CallbackQuery.Message == nil {
		return
	}

	channelID, flag, ok := parseSetFlagPayload(update.CallbackQuery.Data)
	if !ok {
		d.telegramBot.CallbackResponse(
			ctx,
			guard.CallbackResponse{ID: update.CallbackQuery.ID, Text: "Bad toggle payload", ShowAlert: true},
		)
		return
	}

	if err := d.applyFlagToggle(ctx, channelID, flag); err != nil {
		d.log.Errorf("toggle %s for %d failed: %s", flag, channelID, err)
		d.telegramBot.CallbackResponse(
			ctx,
			guard.CallbackResponse{ID: update.CallbackQuery.ID, Text: "Toggle failed: " + err.Error(), ShowAlert: true},
		)
		return
	}

	// Re-render the toggle board so the operator sees the new state.
	d.SettingsChannelHandler(ctx, update)
}

// applyFlagToggle performs the actual flag flip, persistence and ticker sync
// under the channels mutex. It is split out so it can be unit-tested without a
// Telegram update.
func (d *Domain) applyFlagToggle(ctx context.Context, channelID int64, flag string) error {
	d.channelsMutex.Lock()
	protectedChannel, ok := d.protectedChannels[channelID]
	if !ok {
		d.channelsMutex.Unlock()
		return fmt.Errorf("channel %d not found", channelID)
	}

	switch flag {
	case flagAutoScan:
		protectedChannel.AutoScan = !protectedChannel.AutoScan
	case flagAutoClean:
		protectedChannel.AutoClean = !protectedChannel.AutoClean
	case flagAllowClean:
		protectedChannel.AllowClean = !protectedChannel.AllowClean
	default:
		d.channelsMutex.Unlock()
		return fmt.Errorf("unknown flag %q", flag)
	}

	// Persist the updated channel record (load-modify-save is not needed: Store
	// overwrites the whole hash entry).
	if d.storage != nil {
		storeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		err := d.storage.Store(storeCtx, storageRecord(protectedChannel))
		if err != nil {
			d.channelsMutex.Unlock()
			return fmt.Errorf("persist channel: %w", err)
		}
	}

	d.protectedChannels[channelID] = protectedChannel

	d.channelsMutex.Unlock()

	// The schedule is not touched here. The ticker loop reads these flags on
	// every tick, so switching automation off simply makes the channel's turn
	// a no-op, and switching it back on resumes the channel's own schedule
	// instead of replacing it with a fresh interval ticker.
	d.log.Infof("toggled %s for channel %d -> autoscan=%t autoclean=%t allowclean=%t",
		flag, channelID, protectedChannel.AutoScan, protectedChannel.AutoClean, protectedChannel.AllowClean)
	return nil
}

// parseSetFlagPayload parses "/setflag <id> <flag>" into its parts.
func parseSetFlagPayload(data string) (channelID int64, flag string, ok bool) {
	parts := strings.Split(data, " ")
	if len(parts) < 3 || parts[0] != "/setflag" {
		return 0, "", false
	}
	id, ok := parseChannelArg(data)
	if !ok {
		return 0, "", false
	}
	return id, parts[2], true
}
