package cached

import (
	"context"
)

func (d *Domain) JoinChannelByInviteLink(
	ctx context.Context,
	link string,
) error {

	return d.userBot.JoinChannelByInviteLink(ctx, link)
}
