package telegram_bot

import (
	"context"
)

func (d *Domain) Run(
	ctx context.Context,
) error {
	
	go d.botClient.Start(ctx)
	return nil
}
