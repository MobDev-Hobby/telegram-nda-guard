package user_checker_cached_wrap

import (
	"context"

	"github.com/gotd/td/tg"
)

type UserCheckerDomain interface {
	HasAccess(
		ctx context.Context,
		user tg.User,
	) bool
}
