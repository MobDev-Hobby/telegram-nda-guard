package user_checker_cached_wrap

import "time"

type Domain struct {
	checker UserCheckerDomain
	cache   map[int64]Cache
	ttl     time.Duration
}

func New(checker UserCheckerDomain, ttl time.Duration) *Domain {
	return &Domain{
		checker: checker,
		cache: make(map[int64]Cache),
		ttl: ttl,
	}
}
