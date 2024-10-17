package bot

import (
	"context"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	guard "github.com/MobDev-Hobby/telegram-nda-guard"
	"github.com/MobDev-Hobby/telegram-nda-guard/utils"
)

func (d *Domain) SendMessage(ctx context.Context, message *guard.Message) error {

	tgMessage := &bot.SendMessageParams{
		ChatID:          message.ChatID,
		Text:            message.Text,
		ParseMode:       models.ParseModeHTML,
		MessageThreadID: utils.UnPtr(message.ThreadID),
		LinkPreviewOptions: &models.LinkPreviewOptions{
			IsDisabled: utils.Ptr(false),
		},
	}

	var inlineButtons [][]models.InlineKeyboardButton
	for _, buttonsLine := range message.InlineButtons {
		buttonsRow := []models.InlineKeyboardButton{}
		for _, button := range buttonsLine {
			buttonsRow = append(buttonsRow, models.InlineKeyboardButton{
				Text:         button.Text,
				CallbackData: button.Command,
			})
		}
		inlineButtons = append(inlineButtons, buttonsRow)
	}
	if len(inlineButtons) > 0 {
		tgMessage.ReplyMarkup = &models.InlineKeyboardMarkup{
			InlineKeyboard: inlineButtons,
		}
	}

	var buttons [][]models.KeyboardButton
	for _, buttonsLine := range message.Buttons {
		buttonsRow := []models.KeyboardButton{}
		for _, button := range buttonsLine {
			buttonEntity := models.KeyboardButton{
				Text: button.Text,
			}
			if button.RequestChannel != nil && *button.RequestChannel {
				buttonEntity.RequestChat = d.sendAddChannelButton(ctx, button.ID)
			}
			buttonsRow = append(buttonsRow, buttonEntity)
		}
		buttons = append(buttons, buttonsRow)
	}
	if len(buttons) > 0 {
		tgMessage.ReplyMarkup = &models.ReplyKeyboardMarkup{
			Keyboard: buttons,
		}
	}

	needClean := false
	if tgMessage.Text == "" {
		needClean = true
		tgMessage.Text = "Processing..."
	}
	msg, err := d.botClient.SendMessage(
		ctx, tgMessage,
	)

	if needClean {
		_, _ = d.botClient.DeleteMessage(ctx, &bot.DeleteMessageParams{
			ChatID:    tgMessage.ChatID,
			MessageID: msg.ID,
		})
	}

	return err
}

func (d *Domain) sendAddChannelButton(_ context.Context, id int32) *models.KeyboardButtonRequestChat {

	return &models.KeyboardButtonRequestChat{
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
}
