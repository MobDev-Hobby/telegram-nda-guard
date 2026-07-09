package ratelimited

import (
	"context"
	"errors"
	"time"

	"github.com/go-telegram/bot"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

// UserRestrictor is the subset of bot capabilities needed to ban/unban users
// and send report messages. The bot.Domain (telegram/bots/bot) implements it
// via its Ban/Unban/SendReportMessage methods.
type UserRestrictor interface {
	Ban(ctx context.Context, channelID, userID int64, revokeMessages bool) error
	Unban(ctx context.Context, channelID, userID int64) error
	SendReportMessage(ctx context.Context, chatID int64, text string) error
}

// Restrictor is a rate-limited, FLOOD_WAIT-aware adapter over a UserRestrictor.
//
// It exists because the kicker previously called BanChatMember/UnbanChatMember
// on the raw *bot.Bot, bypassing throttling entirely — the most reliable way
// to get the bot account rate-limited or banned. It also turns Telegram 429
// responses into a bounded sleep+retry instead of a silent per-user failure.
type Restrictor struct {
	log          Logger
	botClient    UserRestrictor
	rateLimit    *rate.Limiter
	retryTimeout func(retryAfterSec int) time.Duration
}

// RestrictorOption configures a Restrictor.
type RestrictorOption func(*Restrictor)

// WithRestrictorLogger sets the logger.
func WithRestrictorLogger(logger Logger) RestrictorOption {
	return func(r *Restrictor) {
		r.log = logger
	}
}

// NewRestrictor wraps a UserRestrictor with rate limiting and FLOOD_WAIT
// handling. interval is the token smoothing window; maxBurst is the maximum
// number of ban/unban calls allowed at once.
func NewRestrictor(
	client UserRestrictor,
	interval time.Duration,
	maxBurst int,
	opts ...RestrictorOption,
) *Restrictor {
	r := &Restrictor{
		log:          Logger(zap.NewNop().Sugar()),
		botClient:    client,
		rateLimit:    rate.NewLimiter(rate.Every(interval), maxBurst),
		retryTimeout: defaultFloodWait,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Ban throttles and then bans userID in channelID, retrying once on a 429.
func (r *Restrictor) Ban(ctx context.Context, channelID, userID int64, revokeMessages bool) error {
	if err := r.rateLimit.Wait(ctx); err != nil {
		return err
	}
	err := r.botClient.Ban(ctx, channelID, userID, revokeMessages)
	if retryAfter, ok := floodWaitSeconds(err); ok {
		r.log.Warnf("Ban FLOOD_WAIT, sleeping %ds before retry", retryAfter)
		if !sleepOrCancel(ctx, r.retryTimeout(retryAfter)) {
			return ctx.Err()
		}
		return r.botClient.Ban(ctx, channelID, userID, revokeMessages)
	}
	return err
}

// Unban throttles and then lifts the ban on userID in channelID. It retries
// once on a 429. OnlyIfBanned:true is applied inside the wrapped client so a
// preceding failed ban does not produce a spurious error.
func (r *Restrictor) Unban(ctx context.Context, channelID, userID int64) error {
	if err := r.rateLimit.Wait(ctx); err != nil {
		return err
	}
	err := r.botClient.Unban(ctx, channelID, userID)
	if retryAfter, ok := floodWaitSeconds(err); ok {
		r.log.Warnf("Unban FLOOD_WAIT, sleeping %ds before retry", retryAfter)
		if !sleepOrCancel(ctx, r.retryTimeout(retryAfter)) {
			return ctx.Err()
		}
		return r.botClient.Unban(ctx, channelID, userID)
	}
	return err
}

// SendReport throttles and then sends a report message.
func (r *Restrictor) SendReport(ctx context.Context, chatID int64, text string) error {
	if err := r.rateLimit.Wait(ctx); err != nil {
		return err
	}
	return r.botClient.SendReportMessage(ctx, chatID, text)
}

// floodWaitSeconds returns the RetryAfter value embedded in a go-telegram/bot
// *TooManyRequestsError (HTTP 429), if any.
func floodWaitSeconds(err error) (int, bool) {
	var tooMany *bot.TooManyRequestsError
	if err != nil && errors.As(err, &tooMany) {
		return tooMany.RetryAfter, true
	}
	return 0, false
}

// defaultFloodWait converts a RetryAfter (seconds) into a sleep duration,
// clamped to a sane ceiling so a single 429 cannot stall the worker for an
// hour. One second is the floor so we always yield on a retry.
func defaultFloodWait(retryAfterSec int) time.Duration {
	d := time.Duration(retryAfterSec) * time.Second
	if d > 60*time.Second {
		d = 60 * time.Second
	}
	if d < time.Second {
		d = time.Second
	}
	return d
}

// sleepOrCancel sleeps for d unless ctx is cancelled; returns false if the
// context expired during the sleep.
func sleepOrCancel(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}
