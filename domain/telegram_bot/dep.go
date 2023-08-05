package telegram_bot

import (
	"context"

	"github.com/MobDev-Hobby/telegram-nda-guard/interfaces"
)

//go:generate mockgen -source dep.go -destination ./dep_mock_test.go -package ${GOPACKAGE}

type Logger interfaces.Logger

type UserBotProvider[
	UserBotGenericInterface UserBotInstance,
	BotAuthenticatorInterface BotAuthenticator,
] interface {
	NewBot(
		ctx context.Context,
		name string,
		authenticator BotAuthenticatorInterface,
	) UserBotGenericInterface
}

type UsernameInterface interface {
	Editable() bool
	Active() bool
	Username() string
}

type UserInterface interface {
	UserInfoInterface
	ID() int64
	AccessHash() int64
	Self() bool
	Deleted() bool
	Bot() bool
	Username() string
	Phone() string
	Usernames() []UsernameInterface
}

type UserInfoInterface interface {
	GetFirstName() string
	GetLastName() string
}

type UserBotInstance interface {
	GetChannelUsers(
		ctx context.Context,
		channelId int64,
	) ([]UserInterface, error)
}

type BotAuthenticator interface {
	Phone(ctx context.Context) (string, error)
	Password(ctx context.Context) (string, error)
	Code(ctx context.Context) (string, error)
	SignUp(ctx context.Context) (*UserInfo, error)
	Done()
}
