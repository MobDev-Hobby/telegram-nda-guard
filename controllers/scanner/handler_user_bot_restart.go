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
	// Check the send result before re-initializing. Previously
	// d.runUserBot(ctx) ran unconditionally and the error check below was
	// effectively dead code.
	if err != nil {
		d.log.Errorf("can't send message: %s", err)
		return
	}
	d.runUserBot(ctx)

	d.log.Debugf("processed get ID for chat: %d", update.Message.ChatID)
}
