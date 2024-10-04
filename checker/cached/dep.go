package cached

import (
	"context"

	guard "github.com/MobDev-Hobby/telegram-nda-guard"
)

type UserCheckerDomain interface {
	HasAccess(
		ctx context.Context,
		user *guard.User,
	) (bool, error)
}
