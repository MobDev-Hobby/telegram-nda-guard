package telegram_bot_send_message_ratelimited

import (
	"context"

	"github.com/MobDev-Hobby/telegram-nda-guard/interfaces"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

//go:generate mockgen -source dep.go -destination ./dep_mock_test.go -package ${GOPACKAGE}

type Logger interfaces.Logger

type TelegramBotMessageSender interface {
	SendMessage(
		ctx context.Context,
		params *bot.SendMessageParams,
	) (*models.Message, error)
}
