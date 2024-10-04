package cached

import "time"

type Option func(domain *Domain)

func WithTTL(ttl time.Duration) Option {
	return func(domain *Domain) {
		domain.ttl = ttl
	}
}
