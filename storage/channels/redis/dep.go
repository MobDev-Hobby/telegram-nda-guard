package redis

import (
	"context"
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
	IsNil(err error) bool
	DropHashValue(ctx context.Context, hash string, key string) error
	SetHashValue(ctx context.Context, hash string, key string, value []byte, expiration time.Duration) error
	GetAllHashValues(ctx context.Context, hash string) (map[string][]byte, error)
}
