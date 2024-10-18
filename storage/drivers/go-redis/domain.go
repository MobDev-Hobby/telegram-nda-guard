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

func (d *Domain) IsNil(err error) bool {
	return err == redis.Nil
}

func (d *Domain) DropHashValue(ctx context.Context, hash string, key string) error {
	return d.Client.HDel(ctx, hash, key).Err()
}

func (d *Domain) SetHashValue(ctx context.Context, hash string, key string, value []byte, expiration time.Duration) error {
	return d.Client.HSet(ctx, hash, key, value).Err()
}

func (d *Domain) GetAllHashValues(ctx context.Context, hash string) (map[string][]byte, error) {
	result, err := d.Client.HGetAll(ctx, hash).Result()
	if err != nil {
		return nil, err
	}

	out := make(map[string][]byte, len(result))
	for k, v := range result {
		out[k] = []byte(v)
	}
	return out, nil
}
