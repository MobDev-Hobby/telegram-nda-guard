package example_processor

import (
	"context"

	"github.com/Mobdev-Hobby/telegram-nda-guard/internal/domain/session_storage"
	"github.com/Mobdev-Hobby/telegram-nda-guard/internal/domain/user_bot"
	"github.com/go-telegram/bot"
	"github.com/gotd/td/tg"
)

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
		sessionStorage user_bot.SessionStorage,
		authenticator user_bot.Authenticator,
	) user_bot.UserBot
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
