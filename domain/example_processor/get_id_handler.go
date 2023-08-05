package example_processor

import (
	"context"
	"fmt"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func (d *Domain) getIdHandler(
	ctx context.Context,
	botClient *bot.Bot,
	update *models.Update,
) {

	d.log.Debugf("process get id for chat: %d", update.Message.Chat.ID)

	_, err := botClient.SendMessage(
		ctx, &bot.SendMessageParams{
			ChatID:          update.Message.Chat.ID,
			MessageThreadID: update.Message.MessageThreadID,
			Text: fmt.Sprintf(
				"Chat id is: %d",
				update.Message.Chat.ID,
			),
		},
	)

	if err != nil {
		d.log.Errorf("can't send message: %s", err)
		return
	}

	d.log.Debugf("processed get id for chat: %d", update.Message.Chat.ID)
}
