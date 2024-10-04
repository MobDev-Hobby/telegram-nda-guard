package reporter

import (
	"context"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type Logger interface {
	Panicf(template string, args ...any)
	Errorf(template string, args ...any)
	Warnf(template string, args ...any)
	Infof(template string, args ...any)
	Debugf(template string, args ...any)
}

type TelegramBotMessageSender interface {
	SendMessage(
		ctx context.Context,
		params *bot.SendMessageParams,
	) (*models.Message, error)
}
