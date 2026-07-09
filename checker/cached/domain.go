package cached

import (
	"sync"
	"time"
)

type Domain struct {
	checker UserCheckerDomain
	cache   map[int64]Cache
	// mu guards cache. HasAccess is called concurrently from the scanner
	// worker pool, so the plain map would race and crash the process
	// ("concurrent map read and map write").
	mu  sync.RWMutex
	ttl time.Duration
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
