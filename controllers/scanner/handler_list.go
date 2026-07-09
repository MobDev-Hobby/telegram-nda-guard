package scanner

import (
	"context"
	"fmt"

	guard "github.com/MobDev-Hobby/telegram-nda-guard"
)

func (d *Domain) ListChannelsHandler(
	ctx context.Context,
	update *guard.Update,
) {

	d.log.Debugf("Chat list request got from chat: %d", update.Message.ChatID)

	if !d.hasCommandChannels(update.Message.ChatID) {
		err := d.telegramBot.SendMessage(
			ctx, &guard.Message{
				ChatID:   update.Message.ChatID,
				ThreadID: update.Message.ThreadID,
				Text: fmt.Sprintf(
					"No connected chats found for this channel %d, use /add please",
					update.Message.ChatID,
				),
			},
		)
		if err != nil {
			d.log.Errorf("can't send message: %s", err)
			return
		}
		return
	}

	err := d.telegramBot.SendMessage(
		ctx, &guard.Message{
			ChatID:   update.Message.ChatID,
			ThreadID: update.Message.ThreadID,
			Text:     "<b>Your channels</b> /list:\n",
		},
	)

	if err != nil {
		d.log.Errorf("can't send message: %s", err)
		return
	}

	for _, channelID := range d.getCommandChannels(update.Message.ChatID) {

		message := ""
		buttons := []guard.InlineButton{}

		channel, ok1 := d.getChannel(channelID)
		protectedChannel, ok2 := d.getProtectedChannel(channelID)

		switch {
		case !ok1:
			message = fmt.Sprintf("\n• <b>...%d</b> - wait...", channelID)
		case !ok2:
			message = fmt.Sprintf("\n• <b>%s</b>\n - <b>not protected</b>", channel.title)
		default:
			message = fmt.Sprintf("\n• <b>%s</b>", channel.title)
			if channel.CanScan() {

				buttons = append(
					buttons,
					guard.InlineButton{
						Text:    "/scan",
						Command: fmt.Sprintf("/scan %d", channelID),
					},
				)
			}

			if protectedChannel.AllowClean && channel.CanClean() {

				buttons = append(
					buttons,
					guard.InlineButton{
						Text:    "/clean",
						Command: fmt.Sprintf("/clean %d", channelID),
					},
				)
			}

			// Settings is always available so operators can tune flags even
			// before the bot has confirmed scan/clean rights.
			buttons = append(
				buttons,
				guard.InlineButton{
					Text:    "⚙",
					Command: fmt.Sprintf("/settings %d", channelID),
				},
			)
		}

		err = d.telegramBot.SendMessage(
			ctx, &guard.Message{
				ChatID:        update.Message.ChatID,
				ThreadID:      update.Message.ThreadID,
				Text:          message,
				InlineButtons: [][]guard.InlineButton{buttons},
			},
		)
		if err != nil {
			d.log.Errorf("can't send message: %s", err)
			return
		}
	}
}
