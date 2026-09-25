package bot

import (
	"context"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	guard "github.com/MobDev-Hobby/telegram-nda-guard"
)

func castMessage(message *models.Message) *guard.MessageReceived {
	var chatShared *guard.ChatShared
	if message.ChatShared != nil {
		chatShared = &guard.ChatShared{
			ChatID:    message.ChatShared.ChatID,
			RequestID: int(message.ChatShared.RequestID),
		}
	}

	return &guard.MessageReceived{
		Message: guard.Message{
			ChatID:   message.Chat.ID,
			ChatType: string(message.Chat.Type),
			Text:     message.Text,
			ThreadID: &message.MessageThreadID,
		},
		User: guard.User{
			ID:        message.From.ID,
			Username:  message.From.Username,
			FirstName: message.From.FirstName,
			LastName:  message.From.LastName,
		},
		ChatShared: chatShared,
	}
}

func castUpdate(update *models.Update) *guard.Update {
	matchUpdate := &guard.Update{}
	if update.Message != nil {
		matchUpdate.Message = castMessage(update.Message)
	}

	if update.CallbackQuery != nil {
		from := update.CallbackQuery.From
		matchUpdate.CallbackQuery = &guard.CallbackQuery{
			ID:   update.CallbackQuery.ID,
			Data: update.CallbackQuery.Data,
			From: guard.User{
				ID:        from.ID,
				Username:  from.Username,
				FirstName: from.FirstName,
				LastName:  from.LastName,
			},
		}

		if update.CallbackQuery.Message.Message != nil {
			matchUpdate.CallbackQuery.Message = castMessage(update.CallbackQuery.Message.Message)
		}
	}

	if m := update.MyChatMember; m != nil {
		membership := &guard.BotMembership{
			Chat:   guard.ChannelInfo{ID: m.Chat.ID, Title: m.Chat.Title, Type: string(m.Chat.Type)},
			From:   castUser(m.From),
			Status: string(m.NewChatMember.Type),
		}
		if a := m.NewChatMember.Administrator; a != nil {
			membership.CanRestrictMembers = a.CanRestrictMembers
			membership.CanInviteUsers = a.CanInviteUsers
		}
		matchUpdate.MyChatMember = membership
	}

	if r := update.ChatJoinRequest; r != nil {
		matchUpdate.JoinRequest = &guard.JoinRequest{
			Chat: guard.ChannelInfo{ID: r.Chat.ID, Title: r.Chat.Title, Type: string(r.Chat.Type)},
			User: castUser(r.From),
			At:   int64(r.Date),
			Bio:  r.Bio,
		}
	}

	return matchUpdate
}

func castUser(u models.User) guard.User {
	return guard.User{ID: u.ID, Username: u.Username, FirstName: u.FirstName, LastName: u.LastName}
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

func (d *Domain) CallbackResponse(ctx context.Context, response guard.CallbackResponse) {
	// An unanswered callback only leaves a spinner on the button; log and go on.
	_, err := d.botClient.AnswerCallbackQuery(
		ctx,
		&bot.AnswerCallbackQueryParams{
			CallbackQueryID: response.ID,
			Text:            response.Text,
			ShowAlert:       response.ShowAlert,
		})
	if err != nil {
		d.log.Warnf("can't answer callback query: %s", err)
	}
}
