package bot

import (
	"context"

	guard "github.com/MobDev-Hobby/telegram-nda-guard"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func castUpdate(update *models.Update) *guard.Update {
	matchUpdate := &guard.Update{}
	if update.Message != nil {
		matchUpdate.Message = &guard.Message{
			ChatID:   update.Message.Chat.ID,
			ChatType: string(update.Message.Chat.Type),
			Text:     update.Message.Text,
			ThreadID: &update.Message.MessageThreadID,
		}
	}
	return matchUpdate
}
func (d *Domain) RegisterHandler(
	_ context.Context,
	matcher func(update *guard.Update) bool,
	callback func(ctx context.Context, update *guard.Update),
) string {

	return d.botClient.RegisterHandlerMatchFunc(
		func(update *models.Update) bool {
			return matcher(castUpdate(update))
		},
		func(ctx context.Context, bot *bot.Bot, update *models.Update) {
			callback(ctx, castUpdate(update))
		},
	)
}

func (d *Domain) ClearHandler(id string) {
	d.botClient.UnregisterHandler(id)
}
