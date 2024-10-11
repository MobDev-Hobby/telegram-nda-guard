package redis

import (
	"crypto/cipher"
	"crypto/rand"
	"io"

	"go.uber.org/zap"
)

type Domain struct {
	log     Logger
	redis   RedisClient
	cryptor cipher.Block
	keyPrefix    string
	randomReader io.Reader
}

func New(
	cryptor CryproProvider,
	redis RedisClient,
	opts ...SSOpts,
) (*Domain, error) {

	d := &Domain{
		log:          Logger(zap.NewNop().Sugar()),
		keyPrefix:    "tgSession",
		cryptor:      cryptor,
		redis:        redis,
		randomReader: rand.Reader,
	}

	for _, opt := range opts {
		opt(d)
	}

	return d, nil
}
