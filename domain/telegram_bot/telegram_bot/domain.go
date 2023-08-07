package telegram_bot

import (
	"fmt"

	"github.com/go-telegram/bot"
	"go.uber.org/zap"
)

type Domain struct {
	log       Logger
	botClient *bot.Bot
}

func New(apiKey string, log Logger) (*Domain, error) {
	logger := Logger(zap.NewNop().Sugar())
	if log != nil {
		logger = log
	}
	botClient, err := bot.New(
		apiKey, []bot.Option{}...,
	)
	if err != nil {
		return nil, fmt.Errorf("bot init error: %w", err)
	}
	return &Domain{
		log:    logger,
		botClient: botClient,
	}, nil
}
