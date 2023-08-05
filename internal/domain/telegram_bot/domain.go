package telegram_bot

import (
	"github.com/go-telegram/bot"
	"go.uber.org/zap"
)

type Domain struct {
	apiKey    string
	log       Logger
	botClient *bot.Bot
}

func New(apiKey string, log Logger) *Domain {
	logger := Logger(zap.NewNop().Sugar())
	if log != nil {
		logger = log
	}
	return &Domain{
		log:    logger,
		apiKey: apiKey,
	}
}
