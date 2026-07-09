package ratelimited

import (
	"sync"
	"time"

	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

type Domain struct {
	log                  Logger
	botClient            TelegramBotMessageSender
	mu                   sync.Mutex
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

// getLimiter returns the per-chat limiter, creating it if absent, under the
// lock. The map is read/written from concurrent SendMessage calls, so it
// must be guarded to avoid a fatal data race.
func (d *Domain) getLimiter(chatID int64) *rate.Limiter {
	d.mu.Lock()
	defer d.mu.Unlock()
	limit, found := d.rateLimitByChannelID[chatID]
	if !found {
		// Telegram limit is 20 messages for 1 chat per minute,
		// take 15 for time window inconsistency risk
		// https://core.telegram.org/bots/faq#my-bot-is-hitting-limits-how-do-i-avoid-this
		limit = rate.NewLimiter(rate.Every(1*time.Minute), 15)
		d.rateLimitByChannelID[chatID] = limit
	}
	return limit
}
