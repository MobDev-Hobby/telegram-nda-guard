package user_checker_cached_wrap

import (
	"context"
	"time"

	"github.com/gotd/td/tg"
)

func (d *Domain) HasAccess(
	ctx context.Context, 
	user tg.User,
) bool {

	if record, found := d.cache[user.ID]; found {
		if record.updated.Add(d.ttl).After(time.Now()) {
			return record.hasAccess
		}
	}

	d.cache[user.ID] = Cache{
		updated: time.Now(),
		hasAccess: d.checker.HasAccess(ctx, user),
	}
	
	return d.cache[user.ID].hasAccess
}
