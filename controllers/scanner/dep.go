package scanner

import (
	"context"

	guard "github.com/MobDev-Hobby/telegram-nda-guard"
	"github.com/MobDev-Hobby/telegram-nda-guard/processors"
	"github.com/MobDev-Hobby/telegram-nda-guard/storage/channels"
)

type ProtectedChannelStorage interface {
	LoadAll(ctx context.Context) ([]channels.ProtectedChannel, error)
	Store(ctx context.Context, protectedChannel *channels.ProtectedChannel) error
}

type Logger interface {
	Panicf(template string, args ...any)
	Errorf(template string, args ...any)
	Warnf(template string, args ...any)
	Infof(template string, args ...any)
	Debugf(template string, args ...any)
}

type UserReportProcessor interface {
	ProcessReport(ctx context.Context, report processors.AccessReport)
}

type CheckUserAccess interface {
	HasAccess(context.Context, *guard.User) (bool, error)
}

type UserBot interface {
	Run(
		ctx context.Context,
	) error
	GetChannelUsers(
		ctx context.Context,
		channelID int64,
	) ([]guard.User, error)
	UserID() int64
	Username() string
}

type TelegramBot interface {
	Run(ctx context.Context) error
	// GetBot() *bot.Bot
	GetChat(ctx context.Context, id int64) (*guard.ChannelInfo, error)
	UserID() int64
	Username() string
	GetInviteLink(ctx context.Context, channelID int64) (string, error)
	RegisterHandler(
		ctx context.Context,
		matcher func(update *guard.Update) bool,
		callback func(ctx context.Context, update *guard.Update),
	) string
	ClearHandler(string)
	SendMessage(ctx context.Context, message *guard.Message) error
	CheckAccessUser(ctx context.Context, channelID int64, userID int64, options ...guard.Permission) (
		bool,
		map[guard.Permission]bool,
		error,
	)
	PromoteUser(ctx context.Context, channelID int64, userID int64, options ...guard.Permission) (
		bool,
		error,
	)
	CallbackResponse(ctx context.Context, response guard.CallbackResponse)
}
