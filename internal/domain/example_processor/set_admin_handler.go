package example_processor

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func (d *Domain) getSetAdminHandlerWithCallback(
	callback func(),
) func(
	ctx context.Context,
	botClient *bot.Bot,
	update *models.Update,
) {
	return func(
		ctx context.Context,
		botClient *bot.Bot,
		update *models.Update,
	) {

		d.log.Debugf("process set admin for chat: %d", update.Message.Chat.ID)

		command := strings.Split(update.Message.Text, " ")
		if len(command) < 2 || command[1] != d.setAdminHash {
			_, err := botClient.SendMessage(
				ctx, &bot.SendMessageParams{
					ChatID:          update.Message.Chat.ID,
					MessageThreadID: update.Message.MessageThreadID,
					Text: fmt.Sprintf(
						"There is no way for you :)))",
					),
				},
			)
			if err != nil {
				d.log.Errorf("can't send message: %s", err)
				return
			}
			return
		}

		if d.adminUserChatId != 0 {
			_, err := botClient.SendMessage(
				ctx, &bot.SendMessageParams{
					ChatID:          update.Message.Chat.ID,
					MessageThreadID: update.Message.MessageThreadID,
					Text: fmt.Sprintf(
						"There is no way for you now :)))",
					),
				},
			)
			if err != nil {
				d.log.Errorf("can't send message: %s", err)
				return
			}
			return
		}

		d.adminUserChatId = update.Message.Chat.ID
		_, err := botClient.SendMessage(
			ctx, &bot.SendMessageParams{
				ChatID:          update.Message.Chat.ID,
				MessageThreadID: update.Message.MessageThreadID,
				Text: fmt.Sprintf(
					"You are admin now, running user bot...",
				),
			},
		)

		if err != nil {
			d.log.Errorf("can't send message: %s", err)
			return
		}

		callback()
		d.log.Debugf("processed set admin for chat: %d", update.Message.Chat.ID)
	}
}
