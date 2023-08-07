package telegram_bot

import (
	"context"
	"fmt"

	"github.com/go-telegram/bot"
)

func (d *Domain) Run(
	ctx context.Context,
) error {

	var err error
	d.botClient, err = bot.New(
		d.apiKey, []bot.Option{}...,
	)
	if err != nil {
		return fmt.Errorf("bot init error: %w", err)
	}
	go d.botClient.Start(ctx)

	return nil
}
