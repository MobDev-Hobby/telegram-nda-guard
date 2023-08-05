package access_checker_demo

import "github.com/gotd/td/tg"

func (d *Domain) HasAccess(user tg.User) bool {
	return user.ID%2 == 1
}
