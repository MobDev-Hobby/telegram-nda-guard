package cached

import "time"

type UserBotCachedOption func(domain *Domain)

func WithLogger(logger Logger) UserBotCachedOption {
	return func(d *Domain) {
		d.log = logger
	}
}

func WithCacheTTL(ttl time.Duration) UserBotCachedOption {
	return func(d *Domain) {
		d.cacheTime = ttl
	}
}
