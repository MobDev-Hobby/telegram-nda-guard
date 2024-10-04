package bot

import (
	"context"

	guard "github.com/MobDev-Hobby/telegram-nda-guard"
	"github.com/MobDev-Hobby/telegram-nda-guard/utils"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func (d *Domain) SendMessage(ctx context.Context, message *guard.Message) error {
	_, err := d.botClient.SendMessage(
		ctx, &bot.SendMessageParams{
			ChatID:          message.ChatID,
			Text:            message.Text,
			ParseMode:       models.ParseModeHTML,
			MessageThreadID: utils.UnPtr(message.ThreadID),
			LinkPreviewOptions: &models.LinkPreviewOptions{
				IsDisabled: utils.Ptr(false),
			},
		},
	)
	return err
}
