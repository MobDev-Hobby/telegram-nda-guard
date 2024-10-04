package bot

import (
	"context"
	"fmt"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"go.uber.org/zap"
)

type Domain struct {
	log       Logger
	botClient *bot.Bot
	me        *models.User
}

func New(apiKey string, opts ...TelegramBotOption) (*Domain, error) {
	var err error
	initCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	d := &Options{
		initCtx: initCtx,
		Domain: &Domain{
			log: Logger(zap.NewNop().Sugar()),
		},
	}

	for _, opt := range opts {
		opt(d)
	}

	d.botClient, err = bot.New(
		apiKey, []bot.Option{
			bot.WithSkipGetMe(),
		}...,
	)
	if err != nil {
		return nil, fmt.Errorf("bot init error: %w", err)
	}

	d.me, err = d.botClient.GetMe(d.initCtx)
	if err != nil {
		return nil, fmt.Errorf("bot init error: %w", err)
	}

	return d.Domain, nil
}
