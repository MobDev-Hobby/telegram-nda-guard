package processor_access_control_demo

import (
	"context"

	"github.com/MobDev-Hobby/telegram-nda-guard/domain/report_processor"
	"github.com/MobDev-Hobby/telegram-nda-guard/domain/session_storage/session_storage_file"
	"github.com/MobDev-Hobby/telegram-nda-guard/domain/user_bot/user_bot"
	"github.com/MobDev-Hobby/telegram-nda-guard/interfaces"
	"github.com/go-telegram/bot"
	"github.com/gotd/td/tg"
)

type Logger interfaces.Logger

type UserReportProcessor interface {
	ProcessReport(ctx context.Context, report report_processor.AccessReport)
}

type StorageProvider interface {
	GetStorage(
		hash string,
	) *session_storage_file.Storage
}

type CheckUserAccess interface {
	HasAccess(context.Context, tg.User) (bool, error)
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
