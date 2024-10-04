package userbot

import (
	"context"
	"strings"
)

func (d *Domain) JoinChannelByInviteLink(
	ctx context.Context,
	link string,
) error {

	chunks := strings.Split(link, "+")
	_, err := d.userBot.client.API().MessagesImportChatInvite(
		ctx,
		chunks[len(chunks)-1],
	)

	return err
}
