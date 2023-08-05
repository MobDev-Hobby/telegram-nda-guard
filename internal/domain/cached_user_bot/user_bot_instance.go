package cached_user_bot

import (
	"context"

	"github.com/MobDev-Hobby/telegram-nda-guard/internal/domain/user_bot"
)

type CachedBotWrap struct {
	bot UserBot
	d   *Domain
}

func (d *Domain) NewBot(
	ctx context.Context,
	sessionStorage user_bot.SessionStorage,
	authenticator user_bot.Authenticator,
) user_bot.UserBot {
	return &CachedBotWrap{
		d: d,
		bot: d.userBotProvider.NewBot(
			ctx,
			sessionStorage,
			authenticator,
		),
	}
}
