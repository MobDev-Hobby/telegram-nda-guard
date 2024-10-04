package userbot

import (
	"context"
)

type Logger interface {
	Panicf(template string, args ...any)
	Errorf(template string, args ...any)
	Warnf(template string, args ...any)
	Infof(template string, args ...any)
	Debugf(template string, args ...any)
}

type SessionStorage interface {
	LoadSession(ctx context.Context, name string) ([]byte, error)
	StoreSession(ctx context.Context, name string, data []byte) error
}

type UserInfo interface {
	GetFirstName() string
	GetLastName() string
}
