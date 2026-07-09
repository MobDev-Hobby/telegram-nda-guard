package ratelimited

import (
	"context"
	"errors"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
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
	// Always apply the per-chat limiter. Previously the first message to a
	// chat only created the limiter but skipped Wait, bypassing the 20
	// msg/min guard for the very first message of every chat.
	limit := d.getLimiter(chatID)
	d.log.Debugf("Wait channel %d rate limiter", chatID)
	if err := limit.Wait(ctx); err != nil {
		return nil, err
	}
	d.log.Debugf("Allowed by channel %d rate limiter", chatID)

	return d.botClient.SendMessage(ctx, params)
}
