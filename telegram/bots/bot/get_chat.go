package bot

import (
	"context"
	"errors"

	guard "github.com/MobDev-Hobby/telegram-nda-guard"
	"github.com/MobDev-Hobby/telegram-nda-guard/utils"
	"github.com/go-telegram/bot"
)

func (d *Domain) GetChat(ctx context.Context, channelID int64) (*guard.ChannelInfo, error) {
	chat, err := d.GetBot().GetChat(
		ctx, &bot.GetChatParams{
			ChatID: channelID,
		},
	)

	if err != nil {
		return nil, err
	}

	// This one is needed to check chat type
	_, err = d.botClient.GetChatAdministrators(
		ctx,
		&bot.GetChatAdministratorsParams{
			ChatID: channelID,
		},
	)

	var migratedTo *int64
	if err != nil {
		if !bot.IsMigrateError(err) {
			return nil, err
		}

		migrateError := &bot.MigrateError{}
		errors.As(err, &migrateError)
		migratedTo = utils.Ptr(int64(migrateError.MigrateToChatID))
	}

	return &guard.ChannelInfo{
		ID:         channelID,
		Title:      chat.Title,
		MigratedTo: migratedTo,
	}, nil
}
