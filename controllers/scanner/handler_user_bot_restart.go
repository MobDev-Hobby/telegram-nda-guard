package scanner

import (
	"context"

	guard "github.com/MobDev-Hobby/telegram-nda-guard"
)

func (d *Domain) retryHandler(
	ctx context.Context,
	update *guard.Update,
) {

	d.log.Debugf("renew userbot init")
	err := d.telegramBot.SendMessage(
		ctx, &guard.Message{
			ChatID:   update.Message.ChatID,
			ThreadID: update.Message.ThreadID,
			Text:     "Re-init user bot...",
		},
	)
	d.runUserBot(ctx)

	if err != nil {
		d.log.Errorf("can't send message: %s", err)
		return
	}

	d.log.Debugf("processed get ID for chat: %d", update.Message.ChatID)
}
