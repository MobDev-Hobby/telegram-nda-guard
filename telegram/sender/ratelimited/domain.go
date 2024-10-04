package ratelimited

import (
	"time"

	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

type Domain struct {
	log                  Logger
	botClient            TelegramBotMessageSender
	rateLimitByChannelID map[int64]*rate.Limiter
	rateLimitTotal       *rate.Limiter
}

func New(botClient TelegramBotMessageSender, opts ...Option) *Domain {
	d := &Domain{
		log:       Logger(zap.NewNop().Sugar()),
		botClient: botClient,
		// Absolute limit is 30 messages / sec,
		// take 10 messages / sec for decrease the risk
		// https://core.telegram.org/bots/faq#my-bot-is-hitting-limits-how-do-i-avoid-this
		rateLimitTotal:       rate.NewLimiter(rate.Every(1*time.Second), 10),
		rateLimitByChannelID: make(map[int64]*rate.Limiter),
	}
	for _, opt := range opts {
		opt(d)
	}

	return d
}
