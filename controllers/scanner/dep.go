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
	// Drop removes the persisted record of the protected channel identified by
	// channelID. Implementations must be idempotent: dropping a channel that has
	// no persisted record must not return an error.
	Drop(ctx context.Context, channelID int64) error
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

// Authorizer decides whether the sender of an update is allowed to run a
// protected command. Implementations are pluggable via WithAuthorizer; the
// bundled default is a hybrid authorizer (see NewHybridAuthorizer) that allows
// the configured owner and, optionally, administrators of the controlling chat.
//
// When Authorize returns (false, nil) the command is silently skipped. A
// non-nil error also denies the command and is logged.
type Authorizer interface {
	Authorize(ctx context.Context, update *guard.Update) (bool, error)
}

// WebAuthenticator is the transport-neutral counterpart of Authorizer, used by
// non-Telegram surfaces (e.g. the HTTP management API). The callerID is the
// verified Telegram user ID (obtained via the Telegram Login Widget or an
// equivalent token), and scopeChatID is the chat/channel the operation targets
// (0 when there is no chat context, which disables the admin-check branch).
//
// The bundled HybridAuthorizer satisfies both Authorizer and WebAuthenticator,
// so a single configured instance serves both transports.
type WebAuthenticator interface {
	AuthenticateAndAuthorize(ctx context.Context, callerID, scopeChatID int64) (bool, error)
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
	// GetChatAdministrators returns the user IDs of the administrators of chatID
	// (owner and admins). Used by the default authorizer to decide whether the
	// sender of a command may run it.
	GetChatAdministrators(ctx context.Context, chatID int64) ([]int64, error)
	CallbackResponse(ctx context.Context, response guard.CallbackResponse)
}
