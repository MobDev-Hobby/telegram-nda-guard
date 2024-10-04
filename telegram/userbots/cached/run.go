package cached

import (
	"context"

	"github.com/MobDev-Hobby/telegram-nda-guard/telegram/userbots"
)

func (d *Domain) Run(
	ctx context.Context,
	authenticator userbots.Authenticator,
) error {

	return d.userBot.Run(
		ctx,
		authenticator,
	)
}
