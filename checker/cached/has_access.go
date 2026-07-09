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

	// HasAccess is called concurrently from the scanner worker pool, so the
	// cache map must be guarded by a mutex to avoid a fatal data race.
	d.mu.RLock()
	if record, found := d.cache[user.ID]; found {
		if record.updated.Add(d.ttl).After(time.Now()) {
			d.mu.RUnlock()
			return record.hasAccess, nil
		}
	}
	d.mu.RUnlock()

	hasAccess, err := d.checker.HasAccess(ctx, user)
	if nil == err {
		d.mu.Lock()
		d.cache[user.ID] = Cache{
			updated:   time.Now(),
			hasAccess: hasAccess,
		}
		d.mu.Unlock()
	}

	return hasAccess, err
}
