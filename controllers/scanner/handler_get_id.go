package scanner

import (
	"context"
	"fmt"

	guard "github.com/MobDev-Hobby/telegram-nda-guard"
)

func (d *Domain) IDHandler(
	ctx context.Context,
	update *guard.Update,
) {

	d.log.Debugf("process get ID for chat: %d", update.Message.ChatID)

	err := d.telegramBot.SendMessage(
		ctx, &guard.Message{
			ChatID:   update.Message.ChatID,
			ThreadID: update.Message.ThreadID,
			Text: fmt.Sprintf(
				"Chat ID is: <b>%d</b>",
				update.Message.ChatID,
			),
		},
	)

	if err != nil {
		d.log.Errorf("can't send message: %s", err)
		return
	}

	d.log.Debugf("processed get ID for chat: %d", update.Message.ChatID)
}
