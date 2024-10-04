package cached

import (
	"context"
	"time"

	guard "github.com/MobDev-Hobby/telegram-nda-guard"
)

func (d *Domain) HasAccess(
	ctx context.Context,
	user *guard.User,
) (bool, error) {

	if record, found := d.cache[user.ID]; found {
		if record.updated.Add(d.ttl).After(time.Now()) {
			return record.hasAccess, nil
		}
	}

	hasAccess, err := d.checker.HasAccess(ctx, user)
	if nil == err {
		d.cache[user.ID] = Cache{
			updated:   time.Now(),
			hasAccess: hasAccess,
		}
	}

	return hasAccess, err
}
