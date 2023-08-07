package telegram_bot_send_message_ratelimited

import (
	"time"

	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

type Domain struct {
	log                  Logger
	botClient            TelegramBotMessageSender
	rateLimitByChannelId map[int64]*rate.Limiter
	rateLimitTotal       *rate.Limiter
}

func New(botClient TelegramBotMessageSender, log Logger) *Domain {
	logger := Logger(zap.NewNop().Sugar())
	if log != nil {
		logger = log
	}
	return &Domain{
		log:       logger,
		botClient: botClient,
		// Absolute limit is 30 messages / sec, 
		// take 10 messages / sec for decrease the risk
		// https://core.telegram.org/bots/faq#my-bot-is-hitting-limits-how-do-i-avoid-this
		rateLimitTotal: rate.NewLimiter(rate.Every(1 * time.Second), 10),
		rateLimitByChannelId: make(map[int64]*rate.Limiter),
	}
}
