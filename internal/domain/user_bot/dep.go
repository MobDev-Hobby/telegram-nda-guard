package user_bot

import (
	"context"
)

type SessionStorage interface {
	LoadSession(ctx context.Context) ([]byte, error)
	StoreSession(ctx context.Context, data []byte) error
}

type UserInfo interface {
	GetFirstName() string
	GetLastName() string
}
