package scanner

import (
	"context"
	"fmt"

	guard "github.com/MobDev-Hobby/telegram-nda-guard"
)

// AppHandler answers /app with a button that opens the Mini App. The Mini App
// authorizes users itself (per channel), so the command needs no auth.
func (d *Domain) AppHandler(ctx context.Context, update *guard.Update) {
	if update.Message == nil || d.miniAppURL == "" {
		return
	}

	msg := &guard.Message{
		ChatID:   update.Message.ChatID,
		ThreadID: update.Message.ThreadID,
		Text: "Scan your channels, pick who to remove and configure cleaning " +
			"in the NDA Guard app.",
	}
	button := guard.InlineButton{Text: "Open NDA Guard"}
	switch {
	case update.Message.ChatType == guard.ChatTypePrivate:
		button.WebAppURL = d.miniAppURL
	case d.miniAppShortName != "" && d.telegramBot.Username() != "":
		// Telegram rejects web_app buttons outside private chats; a direct
		// Mini App link opens the app in place instead.
		button.URL = fmt.Sprintf("https://t.me/%s/%s", d.telegramBot.Username(), d.miniAppShortName)
	default:
		msg.Text = "Open a private chat with me and send /app there."
		button.Text = "Open private chat"
		button.URL = fmt.Sprintf("https://t.me/%s", d.telegramBot.Username())
	}
	msg.InlineButtons = [][]guard.InlineButton{{button}}

	if err := d.telegramBot.SendMessage(ctx, msg); err != nil {
		d.log.Errorf("can't send app link: %s", err)
	}
}

// setupMiniAppMenu points the bot's menu button in private chats to the Mini
// App, when the bot transport supports it.
func (d *Domain) setupMiniAppMenu(ctx context.Context) {
	if d.miniAppURL == "" {
		return
	}
	menu, ok := d.telegramBot.(interface {
		SetMenuWebApp(ctx context.Context, text, url string) error
	})
	if !ok {
		return
	}
	if err := menu.SetMenuWebApp(ctx, "NDA Guard", d.miniAppURL); err != nil {
		d.log.Errorf("can't set mini app menu button: %s", err)
	}
}
