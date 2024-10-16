package bot

import (
	"context"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	guard "github.com/MobDev-Hobby/telegram-nda-guard"
	"github.com/MobDev-Hobby/telegram-nda-guard/utils"
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

func (d *Domain) SendAddChannelButton(ctx context.Context, message *guard.Message, id int32, buttonText string) error {

	request := models.KeyboardButtonRequestChat{
		RequestID: id,
		UserAdministratorRights: &models.ChatAdministratorRights{
			CanManageChat:      true,
			CanInviteUsers:     true,
			CanRestrictMembers: true,
			CanPromoteMembers:  true,
		},
		BotAdministratorRights: &models.ChatAdministratorRights{
			CanManageChat:      true,
			CanPromoteMembers:  true,
			CanRestrictMembers: true,
		},
	}

	button := &models.KeyboardButton{
		Text:        buttonText,
		RequestChat: &request,
	}

	_, err := d.botClient.SendMessage(
		ctx, &bot.SendMessageParams{
			ChatID:          message.ChatID,
			Text:            message.Text,
			ParseMode:       models.ParseModeHTML,
			MessageThreadID: utils.UnPtr(message.ThreadID),
			LinkPreviewOptions: &models.LinkPreviewOptions{
				IsDisabled: utils.Ptr(false),
			},
			ReplyMarkup: models.ReplyKeyboardMarkup{Keyboard: [][]models.KeyboardButton{{*button}}},
		},
	)
	return err
}
