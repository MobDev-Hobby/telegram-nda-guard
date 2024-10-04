package cached

import (
	"context"

	guard "github.com/MobDev-Hobby/telegram-nda-guard"
	"github.com/MobDev-Hobby/telegram-nda-guard/telegram/userbots"
	"github.com/gotd/td/tg"
)

type Logger interface {
	Panicf(template string, args ...any)
	Errorf(template string, args ...any)
	Warnf(template string, args ...any)
	Infof(template string, args ...any)
	Debugf(template string, args ...any)
}

type UserBot interface {
	Run(
		ctx context.Context,
		authenticator userbots.Authenticator,
	) error
	GetChannelUsers(
		ctx context.Context,
		channelID int64,
	) ([]guard.User, error)
	JoinChannelByInviteLink(
		ctx context.Context,
		link string,
	) error
	UserID() int64
	Username() string
	GetDispatcher() tg.UpdateDispatcher
}
