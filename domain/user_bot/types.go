package user_bot

import "github.com/gotd/td/telegram/auth"

type Authenticator interface {
	auth.UserAuthenticator
	Done()
}
