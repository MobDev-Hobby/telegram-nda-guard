package user_bot_cached_wrap

import (
	"context"

	"github.com/MobDev-Hobby/telegram-nda-guard/domain/user_bot/user_bot"
	"github.com/MobDev-Hobby/telegram-nda-guard/interfaces"
	"github.com/gotd/td/tg"
)

type Logger interfaces.Logger

type UserBot interface {
	GetChannelUsers(
		ctx context.Context,
		channelId int64,
	) ([]tg.User, error)
}

type UserBotProvider interface {
	NewBot(
		ctx context.Context,
		sessionStorage user_bot.SessionStorage,
		authenticator user_bot.Authenticator,
	) user_bot.UserBot
}
