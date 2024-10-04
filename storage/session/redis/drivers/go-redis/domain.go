package goredis

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type Domain struct {
	redis.Client
}

func New(client *redis.Client) *Domain {
	return &Domain{*client}
}

func (d *Domain) Get(ctx context.Context, key string) ([]byte, error) {
	return d.Client.Get(ctx, key).Bytes()
}

func (d *Domain) Set(ctx context.Context, key string, data []byte, ttl time.Duration) error {
	return d.Client.Set(ctx, key, data, ttl).Err()
}
