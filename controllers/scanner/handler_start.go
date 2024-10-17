package scanner

import (
	"context"

	guard "github.com/MobDev-Hobby/telegram-nda-guard"
)

func (d *Domain) startHandler(
	ctx context.Context,
	update *guard.Update,
) {

	d.log.Debugf("Start request got from chat: %d/%s", update.Message.ChatID, update.Message.User.Username)

	err := d.telegramBot.SendMessage(
		ctx, &guard.Message{
			ChatID:   update.Message.ChatID,
			ThreadID: update.Message.ThreadID,
			Text: "Welcome to NDA protector bot!\n" +
				"Commands:\n" +
				"• /add - add protected channels\n" +
				"• /list - see your channels\n" +
				"• /scan - request all your channels scan\n" +
				"• /clean - request all your channels clean\n" +
				"• /help - see help",
			Buttons: d.getDefaultButtons(),
		},
	)
	if err != nil {
		d.log.Errorf("can't send message: %s", err)
		return
	}
}

func (d *Domain) getDefaultButtons() [][]guard.Button {
	return [][]guard.Button{
		{
			{
				Text: "/add - Add channels",
				ID:   1,
			},
		},
		{
			{
				Text: "/list - List channels",
				ID:   2,
			},
		},
	}
}
