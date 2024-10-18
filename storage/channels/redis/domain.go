package redis

import (
	"crypto/rand"
	"io"

	"go.uber.org/zap"
)

type Domain struct {
	log          Logger
	redis        RedisClient
	keyPrefix    string
	randomReader io.Reader
}

func New(
	redis RedisClient,
	opts ...SSOpts,
) (*Domain, error) {

	d := &Domain{
		log:          Logger(zap.NewNop().Sugar()),
		keyPrefix:    "pChannel",
		redis:        redis,
		randomReader: rand.Reader,
	}

	for _, opt := range opts {
		opt(d)
	}

	return d, nil
}
