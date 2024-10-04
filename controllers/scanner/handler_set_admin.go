package scanner

import (
	"context"
	"strings"

	guard "github.com/MobDev-Hobby/telegram-nda-guard"
)

func (d *Domain) getSetAdminHandlerWithCallback(
	callback func(),
) func(
	ctx context.Context,
	update *guard.Update,
) {

	return func(
		ctx context.Context,
		update *guard.Update,
	) {

		d.log.Debugf("process set admin for chat: %d", update.Message.ChatID)

		command := strings.Split(update.Message.Text, " ")
		if len(command) < 2 || d.setAdminHash == nil || command[1] != *d.setAdminHash {
			err := d.telegramBot.SendMessage(
				ctx, &guard.Message{
					ChatID:   update.Message.ChatID,
					ThreadID: update.Message.ThreadID,
					Text:     "There is no way for you :)))",
				},
			)
			if err != nil {
				d.log.Errorf("can't send message: %s", err)
				return
			}
			return
		}

		if d.adminUserChatID != 0 {
			err := d.telegramBot.SendMessage(
				ctx, &guard.Message{
					ChatID:   update.Message.ChatID,
					ThreadID: update.Message.ThreadID,
					Text:     "There is no way for you now :)))",
				},
			)
			if err != nil {
				d.log.Errorf("can't send message: %s", err)
				return
			}
			return
		}

		d.adminUserChatID = update.Message.ChatID
		err := d.telegramBot.SendMessage(
			ctx, &guard.Message{
				ChatID:   update.Message.ChatID,
				ThreadID: update.Message.ThreadID,
				Text:     "You are admin now, running user bot...",
			},
		)

		if err != nil {
			d.log.Errorf("can't send message: %s", err)
			return
		}

		callback()
		d.log.Debugf("processed set admin for chat: %d", update.Message.ChatID)
	}
}
