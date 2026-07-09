package scanner

import (
	"context"

	guard "github.com/MobDev-Hobby/telegram-nda-guard"
)

func (d *Domain) NotifyAdmin(ctx context.Context, msg string) {
	// Guard against the unconfigured sentinel (-1): sending to chat id -1
	// is always rejected by Telegram and only produces noise in the logs.
	if d.adminUserChatID <= 0 {
		return
	}
	err := d.telegramBot.SendMessage(
		ctx, &guard.Message{
			ChatID: d.adminUserChatID,
			Text:   msg,
		},
	)
	if err != nil {
		d.log.Errorf("Error sending admin message: %+v", err)
	}
}
