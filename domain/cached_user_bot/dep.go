package cached_user_bot

import (
	"context"

	user_bot2 "github.com/MobDev-Hobby/telegram-nda-guard/domain/user_bot"
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
		sessionStorage user_bot2.SessionStorage,
		authenticator user_bot2.Authenticator,
	) user_bot2.UserBot
}
