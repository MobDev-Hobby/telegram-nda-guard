package demo

import (
	"context"

	guard "github.com/MobDev-Hobby/telegram-nda-guard"
)

func (d *Domain) HasAccess(
	_ context.Context,
	user *guard.User,
) (bool, error) {

	return user.ID%2 == 1, nil
}
