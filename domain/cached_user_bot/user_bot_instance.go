package cached_user_bot

import (
	"context"

	user_bot2 "github.com/MobDev-Hobby/telegram-nda-guard/domain/user_bot"
)

type CachedBotWrap struct {
	bot UserBot
	d   *Domain
}

func (d *Domain) NewBot(
	ctx context.Context,
	sessionStorage user_bot2.SessionStorage,
	authenticator user_bot2.Authenticator,
) user_bot2.UserBot {
	return &CachedBotWrap{
		d: d,
		bot: d.userBotProvider.NewBot(
			ctx,
			sessionStorage,
			authenticator,
		),
	}
}
