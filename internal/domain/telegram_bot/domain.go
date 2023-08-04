package telegram_bot

import (
	"github.com/go-telegram/bot"
	"go.uber.org/zap"
)

type Domain struct {
	apiKey    string
	log       *zap.SugaredLogger
	botClient *bot.Bot
}

func New(apiKey string, log *zap.SugaredLogger) *Domain {
	logger := zap.NewNop().Sugar()
	if log != nil {
		logger = log
	}
	return &Domain{
		log:    logger,
		apiKey: apiKey,
	}
}
