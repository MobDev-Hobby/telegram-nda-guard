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
		// Telegram allows ~20 messages per chat per minute; we cap at 15 for
		// clock-skew safety. rate.Every sets the per-token refill interval,
		// so for "15 per minute" the interval is a minute divided by 15.
		// Using rate.Every(time.Minute) would refill only 1 token/min, i.e.
		// stall at ~1 msg/min after the burst.
		// https://core.telegram.org/bots/faq#my-bot-is-hitting-limits-how-do-i-avoid-this
		limit = rate.NewLimiter(rate.Every(time.Minute/15), 15)
		d.rateLimitByChannelID[chatID] = limit
	}
	return limit
}
