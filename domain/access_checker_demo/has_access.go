package access_checker_demo

import (
	"context"

	"github.com/gotd/td/tg"
)

func (d *Domain) HasAccess(_ context.Context, user tg.User) bool {
	return user.ID%2 == 1
}
