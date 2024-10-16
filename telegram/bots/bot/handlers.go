package bot

import (
	"context"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	guard "github.com/MobDev-Hobby/telegram-nda-guard"
)

func castUpdate(update *models.Update) *guard.Update {
	matchUpdate := &guard.Update{}
	if update.Message != nil {

		var chatShared *guard.ChatShared
		if update.Message.ChatShared != nil {
			chatShared = &guard.ChatShared{
				ChatID:    update.Message.ChatShared.ChatID,
				RequestID: int(update.Message.ChatShared.RequestID),
			}
		}

		matchUpdate.Message = &guard.MessageReceived{
			Message: guard.Message{
				ChatID:   update.Message.Chat.ID,
				ChatType: string(update.Message.Chat.Type),
				Text:     update.Message.Text,
				ThreadID: &update.Message.MessageThreadID,
			},
			User: guard.User{
				ID:        update.Message.From.ID,
				Username:  update.Message.From.Username,
				FirstName: update.Message.From.FirstName,
				LastName:  update.Message.From.LastName,
			},
			ChatShared: chatShared,
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
