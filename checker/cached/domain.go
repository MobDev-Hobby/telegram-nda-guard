package cached

import "time"

type Domain struct {
	checker UserCheckerDomain
	cache   map[int64]Cache
	ttl     time.Duration
}

func New(checker UserCheckerDomain, opts ...Option) *Domain {
	d := &Domain{
		checker: checker,
		cache:   make(map[int64]Cache),
		ttl:     time.Minute * 60,
	}
	for _, opt := range opts {
		opt(d)
	}
	return d
}
