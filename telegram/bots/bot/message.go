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
			inlineButton := models.InlineKeyboardButton{Text: button.Text}
			switch {
			case button.WebAppURL != "":
				inlineButton.WebApp = &models.WebAppInfo{URL: button.WebAppURL}
			case button.URL != "":
				inlineButton.URL = button.URL
			default:
				inlineButton.CallbackData = button.Command
			}
			buttonsRow = append(buttonsRow, inlineButton)
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
				buttonEntity.RequestChat = requestChatButton(button.ID, button.RequestChatIsChannel)
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

	if needClean && msg != nil {
		_, _ = d.botClient.DeleteMessage(ctx, &bot.DeleteMessageParams{
			ChatID:    tgMessage.ChatID,
			MessageID: msg.ID,
		})
	}

	return err
}

// requestChatButton builds a request_chat keyboard button.
//
// chat_is_channel has no omitempty in go-telegram/bot, so it is always sent:
// false makes Telegram list only groups and supergroups, true only broadcast
// channels. Before this was parameterised the bot always sent false, which is
// why channels could never be picked.
//
// The requested rights are the minimum the guard needs: restrict members (to
// kick) and invite users (to check invite rights). can_promote_members was
// requested before; Telegram then lists only chats where the user may add
// admins, which in practice hides everything the user does not own.
func requestChatButton(id int32, isChannel bool) *models.KeyboardButtonRequestChat {
	rights := &models.ChatAdministratorRights{
		CanRestrictMembers: true,
		CanInviteUsers:     true,
	}
	return &models.KeyboardButtonRequestChat{
		RequestID:               id,
		ChatIsChannel:           isChannel,
		UserAdministratorRights: rights,
		BotAdministratorRights:  rights,
		RequestTitle:            true,
	}
}
