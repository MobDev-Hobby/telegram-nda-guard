package cached

import (
	"context"

	"github.com/gotd/td/tg"

	guard "github.com/MobDev-Hobby/telegram-nda-guard"
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
	) error
	GetChannelUsers(
		ctx context.Context,
		channelID int64,
	) ([]guard.User, error)
	UserID() int64
	Username() string
	GetDispatcher() tg.UpdateDispatcher
}
