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
			Text:     d.helpText(),
			Buttons:  d.getDefaultButtons(),
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

func (d *Domain) helpText() string {
	text := "Welcome to NDA protector bot!\n" +
		"I protect channels and groups: I check their members and remove those " +
		"who have no access.\n\n" +
		"Commands:\n" +
		"• /add - add a protected channel or group\n" +
		"• /list - see your channels\n" +
		"• /settings - change channel settings\n" +
		"• /users &lt;id&gt; - list channel members by access\n" +
		"• /scan - request all your channels scan\n" +
		"• /clean - request all your channels clean\n"
	if d.miniAppURL != "" {
		text += "• /app - open the app: scan and pick who to remove\n"
	}
	return text + "• /help - see help"
}
