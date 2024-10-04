package redis

import (
	"context"
	"crypto/cipher"
	"time"
)

//go:generate mockgen -source dep.go -destination ./dep_mock_test.go -package ${GOPACKAGE}

type Logger interface {
	Panicf(template string, args ...any)
	Errorf(template string, args ...any)
	Warnf(template string, args ...any)
	Infof(template string, args ...any)
	Debugf(template string, args ...any)
}

type RedisClient interface {
	Set(ctx context.Context, key string, value []byte, expiration time.Duration) error
	Get(ctx context.Context, key string) ([]byte, error)
}

type CryproProvider interface {
	cipher.Block
}
