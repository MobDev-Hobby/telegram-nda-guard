package user_bot

import (
	"context"

	"github.com/MobDev-Hobby/telegram-nda-guard/interfaces"
)

type Logger interfaces.Logger

type SessionStorage interface {
	LoadSession(ctx context.Context) ([]byte, error)
	StoreSession(ctx context.Context, data []byte) error
}

type UserInfo interface {
	GetFirstName() string
	GetLastName() string
}
