package user_bot_cached_wrap

import (
	"context"

	"github.com/MobDev-Hobby/telegram-nda-guard/domain/user_bot/user_bot"
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
