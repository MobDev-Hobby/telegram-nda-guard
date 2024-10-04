package scanner

import (
	"context"

	guard "github.com/MobDev-Hobby/telegram-nda-guard"
)

func (d *Domain) NotifyAdmin(ctx context.Context, msg string) {
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
