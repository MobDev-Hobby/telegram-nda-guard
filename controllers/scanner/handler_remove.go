package scanner

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	guard "github.com/MobDev-Hobby/telegram-nda-guard"
)

// RemoveChannelHandler starts a two-step removal flow. Triggered by
// "/remove <id>" — it shows a confirmation prompt with an inline "Confirm
// remove" button. The actual removal only happens in RemoveConfirmHandler.
//
// Two-step confirmation is intentional: removing a channel is destructive and
// should not be triggered by a stray tap.
func (d *Domain) RemoveChannelHandler(
	ctx context.Context,
	update *guard.Update,
) {
	if update.Message == nil {
		return
	}
	commandChatID := update.Message.ChatID

	channelID, ok := parseRemoveChannelArg(update.Message.Text)
	if !ok {
		_ = d.telegramBot.SendMessage(
			ctx, &guard.Message{
				ChatID:   commandChatID,
				ThreadID: update.Message.ThreadID,
				Text:     "Usage: /remove <channel_id> (pick a channel from /list)",
			},
		)
		return
	}

	title, found := d.channelTitleForRemove(channelID, commandChatID)
	if !found {
		_ = d.telegramBot.SendMessage(
			ctx, &guard.Message{
				ChatID:   commandChatID,
				ThreadID: update.Message.ThreadID,
				Text:     fmt.Sprintf("Channel %d is not managed from this chat", channelID),
			},
		)
		return
	}

	_ = d.telegramBot.SendMessage(
		ctx, &guard.Message{
			ChatID:   commandChatID,
			ThreadID: update.Message.ThreadID,
			Text: fmt.Sprintf(
				"Remove <b>%s</b>?\nThis stops protecting the channel from this control chat. "+
					"Confirm to proceed.",
				title,
			),
			InlineButtons: [][]guard.InlineButton{{
				{
					Text:    "🗑 Confirm remove",
					Command: fmt.Sprintf("/rmconfirm %d", channelID),
				},
			}},
		},
	)
}

// RemoveConfirmHandler performs the actual removal. Triggered by the
// "/rmconfirm <id>" inline button. It delegates to CleanProtectedChannel, which
// detaches the channel from the controlling chat and persists the change (drop
// or update) via the storage layer.
func (d *Domain) RemoveConfirmHandler(
	ctx context.Context,
	update *guard.Update,
) {
	if update.CallbackQuery == nil || update.CallbackQuery.Message == nil {
		return
	}
	commandChatID := update.CallbackQuery.Message.ChatID

	channelID, ok := parseRemoveChannelArg(update.CallbackQuery.Data)
	if !ok {
		d.telegramBot.CallbackResponse(
			ctx,
			guard.CallbackResponse{ID: update.CallbackQuery.ID, Text: "Bad remove payload", ShowAlert: true},
		)
		return
	}

	title := strconv.FormatInt(channelID, 10)
	if ch, found := d.channels[channelID]; found {
		title = ch.title
	}

	if err := d.CleanProtectedChannel(channelID, commandChatID); err != nil {
		d.log.Errorf("remove channel %d failed: %s", channelID, err)
		d.telegramBot.CallbackResponse(
			ctx,
			guard.CallbackResponse{ID: update.CallbackQuery.ID, Text: "Remove failed: " + err.Error(), ShowAlert: true},
		)
		return
	}

	d.telegramBot.CallbackResponse(
		ctx,
		guard.CallbackResponse{ID: update.CallbackQuery.ID, Text: "Removed"},
	)
	_ = d.telegramBot.SendMessage(
		ctx, &guard.Message{
			ChatID:   commandChatID,
			ThreadID: update.CallbackQuery.Message.ThreadID,
			Text:     fmt.Sprintf("Channel <b>%s</b> removed.", title),
		},
	)
	d.log.Infof("removed channel %d from control chat %d", channelID, commandChatID)
}

// channelTitleForRemove returns the title of channelID if it is managed from
// commandChatID. Returns found=false otherwise (so callers can refuse removal
// by chats that don't control the channel).
func (d *Domain) channelTitleForRemove(channelID, commandChatID int64) (string, bool) {
	controlled := false
	for _, id := range d.commandChannels[commandChatID] {
		if id == channelID {
			controlled = true
			break
		}
	}
	if !controlled {
		return "", false
	}
	if ch, ok := d.channels[channelID]; ok {
		return ch.title, true
	}
	return strconv.FormatInt(channelID, 10), true
}

// parseRemoveChannelArg extracts the integer channel id from "/remove <id>" or
// "/rmconfirm <id>".
func parseRemoveChannelArg(data string) (int64, bool) {
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
