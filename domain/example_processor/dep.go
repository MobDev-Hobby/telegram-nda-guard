package example_processor

import (
	"context"

	"github.com/MobDev-Hobby/telegram-nda-guard/domain/session_storage"
	user_bot2 "github.com/MobDev-Hobby/telegram-nda-guard/domain/user_bot"
	"github.com/MobDev-Hobby/telegram-nda-guard/interfaces"
	"github.com/go-telegram/bot"
	"github.com/gotd/td/tg"
)

type Logger interfaces.Logger

type StorageProvider interface {
	GetStorage(
		hash string,
	) *session_storage.Storage
}

type CheckUserAccess interface {
	HasAccess(tg.User) bool
}

type UserBotProvider interface {
	NewBot(
		ctx context.Context,
		sessionStorage user_bot2.SessionStorage,
		authenticator user_bot2.Authenticator,
	) user_bot2.UserBot
}

type UserBot interface {
	GetChannelUsers(
		ctx context.Context,
		channelId int64,
	) ([]tg.User, error)
}

type TelegramBotProvider interface {
	Run(ctx context.Context) error
	GetBot() *bot.Bot
}
