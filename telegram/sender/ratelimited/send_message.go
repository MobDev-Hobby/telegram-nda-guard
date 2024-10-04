package ratelimited

import (
	"context"
	"errors"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"golang.org/x/time/rate"
)

func (d *Domain) SendMessage(
	ctx context.Context,
	params *bot.SendMessageParams,
) (*models.Message, error) {

	d.log.Debugf("Wait total rate limiter")
	if err := d.rateLimitTotal.Wait(ctx); err != nil {
		return nil, err
	}
	d.log.Debugf("Allowed by rate limiter")

	chatID, ok := params.ChatID.(int64)
	if !ok {
		return nil, errors.New("chatID is not int64")
	}
	if limit, found := d.rateLimitByChannelID[chatID]; found {
		d.log.Debugf("Wait channel %d rate limiter", chatID)
		if err := limit.Wait(ctx); err != nil {
			return nil, err
		}
		d.log.Debugf("Allowed by channel %d rate limiter", chatID)
	} else {
		// Telegram limit is 20 messages for 1 chat per minute,
		// take 15 for time window inconsistency risk
		// https://core.telegram.org/bots/faq#my-bot-is-hitting-limits-how-do-i-avoid-this
		d.rateLimitByChannelID[chatID] = rate.NewLimiter(
			rate.Every(1*time.Minute),
			15,
		)
	}

	return d.botClient.SendMessage(ctx, params)
}
