package scanner

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	guard "github.com/MobDev-Hobby/telegram-nda-guard"
)

// flag constants for the /setflag callback payload.
const (
	flagAutoScan   = "autoscan"
	flagAutoClean  = "autoclean"
	flagAllowClean = "allowclean"
)

// SettingsHandler renders the list of channels controlled by the originating
// chat, each with a button to open its per-channel settings board. Triggered by
// the /settings command.
func (d *Domain) SettingsHandler(
	ctx context.Context,
	update *guard.Update,
) {
	if update.Message == nil {
		return
	}
	commandChatID := update.Message.ChatID

	d.channelsMutex.RLock()
	channelIDs := append([]int64(nil), d.commandChannels[commandChatID]...)
	d.channelsMutex.RUnlock()

	if len(channelIDs) == 0 {
		err := d.telegramBot.SendMessage(
			ctx, &guard.Message{
				ChatID:   commandChatID,
				ThreadID: update.Message.ThreadID,
				Text:     "No connected chats found for this channel, use /add please",
			},
		)
		if err != nil {
			d.log.Errorf("can't send message: %s", err)
		}
		return
	}

	err := d.telegramBot.SendMessage(
		ctx, &guard.Message{
			ChatID:   commandChatID,
			ThreadID: update.Message.ThreadID,
			Text:     "<b>Channel settings</b> — select a channel:",
		},
	)
	if err != nil {
		d.log.Errorf("can't send message: %s", err)
		return
	}

	for _, channelID := range channelIDs {
		d.channelsMutex.RLock()
		channel, ok := d.channels[channelID]
		d.channelsMutex.RUnlock()
		if !ok {
			continue
		}
		err := d.telegramBot.SendMessage(
			ctx, &guard.Message{
				ChatID:   commandChatID,
				ThreadID: update.Message.ThreadID,
				Text:     fmt.Sprintf("• <b>%s</b>", channel.title),
				InlineButtons: [][]guard.InlineButton{{
					{
						Text:    "⚙ Settings",
						Command: fmt.Sprintf("/settings %d", channelID),
					},
				}},
			},
		)
		if err != nil {
			d.log.Errorf("can't send message: %s", err)
		}
	}
}

// SettingsChannelHandler renders the per-channel toggle board for one channel.
// Triggered by the /settings <id> callback button.
func (d *Domain) SettingsChannelHandler(
	ctx context.Context,
	update *guard.Update,
) {
	if update.CallbackQuery == nil || update.CallbackQuery.Message == nil {
		return
	}

	channelID, ok := parseChannelArg(update.CallbackQuery.Data)
	if !ok {
		d.telegramBot.CallbackResponse(
			ctx,
			guard.CallbackResponse{ID: update.CallbackQuery.ID, Text: "Bad channel id", ShowAlert: true},
		)
		return
	}

	d.channelsMutex.RLock()
	protectedChannel, found := d.protectedChannels[channelID]
	channel, channelFound := d.channels[channelID]
	d.channelsMutex.RUnlock()

	if !found {
		d.telegramBot.CallbackResponse(
			ctx,
			guard.CallbackResponse{ID: update.CallbackQuery.ID, Text: "Channel not found", ShowAlert: true},
		)
		return
	}

	title := strconv.FormatInt(channelID, 10)
	if channelFound {
		title = channel.title
	}

	text := fmt.Sprintf(
		"<b>%s</b> settings\n\nTap a toggle to switch it:",
		title,
	)
	buttons := [][]guard.InlineButton{
		toggleRow(channelID, "Auto Scan", flagAutoScan, protectedChannel.AutoScan),
		toggleRow(channelID, "Auto Clean", flagAutoClean, protectedChannel.AutoClean),
		toggleRow(channelID, "Allow Clean", flagAllowClean, protectedChannel.AllowClean),
	}

	err := d.telegramBot.SendMessage(
		ctx, &guard.Message{
			ChatID:   update.CallbackQuery.Message.ChatID,
			ThreadID: update.CallbackQuery.Message.ThreadID,
			Text:     text,
			InlineButtons: buttons,
		},
	)
	if err != nil {
		d.log.Errorf("can't send settings board: %s", err)
	}
	d.telegramBot.CallbackResponse(ctx, guard.CallbackResponse{ID: update.CallbackQuery.ID})
}

// toggleRow builds a single inline button that reflects the current on/off
// state of a flag and carries the /setflag payload to flip it.
func toggleRow(channelID int64, label, flag string, on bool) []guard.InlineButton {
	state := "OFF"
	if on {
		state = "ON ✅"
	}
	return []guard.InlineButton{{
		Text:    fmt.Sprintf("%s: %s", label, state),
		Command: fmt.Sprintf("/setflag %d %s", channelID, flag),
	}}
}

// parseChannelArg extracts the integer channel id from a callback payload of the
// form "/cmd <id>" or "/cmd <id> <extra>". Returns false on parse failure.
func parseChannelArg(data string) (int64, bool) {
	parts := strings.Split(data, " ")
	if len(parts) < 2 {
		return 0, false
	}
	id, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return 0, false
	}
	return id, true
}
