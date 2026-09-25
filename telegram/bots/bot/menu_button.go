package bot

import (
	"context"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// SetMenuWebApp sets the default menu button of private chats with the bot to
// open the Mini App at url.
func (d *Domain) SetMenuWebApp(ctx context.Context, text, url string) error {
	_, err := d.botClient.SetChatMenuButton(ctx, &bot.SetChatMenuButtonParams{
		MenuButton: models.MenuButtonWebApp{
			Type:   models.MenuButtonTypeWebApp,
			Text:   text,
			WebApp: models.WebAppInfo{URL: url},
		},
	})
	return err
}
