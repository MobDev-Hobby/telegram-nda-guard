package userbots

import (
	"context"

	"github.com/gotd/td/telegram/auth"
)

type Authenticator interface {
	auth.UserAuthenticator
	Done(ctx context.Context)
}

type User struct {
	ID        int64
	Firstname string
	LastName  string
	Username  string
	Phone     string
}
