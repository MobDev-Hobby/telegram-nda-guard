package bot

import (
	"context"

	"github.com/go-telegram/bot"
)

func (d *Domain) GetInviteLink(ctx context.Context, channelID int64) (string, error) {
	link, err := d.GetBot().CreateChatInviteLink(
		ctx,
		&bot.CreateChatInviteLinkParams{
			ChatID:      channelID,
			MemberLimit: 1,
		},
	)

	if err != nil {
		return "", err
	}

	return link.InviteLink, err
}
