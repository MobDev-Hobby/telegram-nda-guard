package report_processor_send_admin_with_telegram_bot

import (
	"context"

	"github.com/MobDev-Hobby/telegram-nda-guard/interfaces"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type Logger interfaces.Logger

type TelegramBotMessageSender interface {
	SendMessage(
		ctx context.Context,
		params *bot.SendMessageParams,
	) (*models.Message, error)
}
