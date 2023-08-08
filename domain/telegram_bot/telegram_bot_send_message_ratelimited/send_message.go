package telegram_bot_send_message_ratelimited

import (
	"context"
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

	chatId := params.ChatID.(int64)
	if limit, found := d.rateLimitByChannelId[chatId]; found {
		d.log.Debugf("Wait channel %d rate limiter", chatId)
		if err := limit.Wait(ctx); err != nil {
			return nil, err
		}
		d.log.Debugf("Allowed by channel %d rate limiter", chatId)
	} else {
		// Telegram limit is 20 messages for 1 chat per minute,
		// take 15 for time window inconsistency risk
		// https://core.telegram.org/bots/faq#my-bot-is-hitting-limits-how-do-i-avoid-this
		d.rateLimitByChannelId[chatId] = rate.NewLimiter(
			rate.Every(1*time.Minute),
			15,
		)
	}

	return d.botClient.SendMessage(ctx, params)
}
