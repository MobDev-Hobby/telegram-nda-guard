// Package redis keeps pending join requests in one Redis hash per chat.
package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/MobDev-Hobby/telegram-nda-guard/storage/joinrequests"
)

// RedisClient matches storage/channels/redis.RedisClient.
type RedisClient interface {
	IsNil(err error) bool
	DropHashValue(ctx context.Context, hash string, key string) error
	SetHashValue(ctx context.Context, hash string, key string, value []byte, expiration time.Duration) error
	GetAllHashValues(ctx context.Context, hash string) (map[string][]byte, error)
}

type Domain struct {
	redis     RedisClient
	keyPrefix string
}

func New(redis RedisClient) *Domain {
	return &Domain{redis: redis, keyPrefix: "pJoinRequests"}
}

func (d *Domain) hashName(chatID int64) string {
	return d.keyPrefix + ":" + strconv.FormatInt(chatID, 10)
}

func (d *Domain) LoadJoinRequests(ctx context.Context, chatID int64) ([]joinrequests.Request, error) {
	raw, err := d.redis.GetAllHashValues(ctx, d.hashName(chatID))
	if err != nil {
		if d.redis.IsNil(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read join requests: %w", err)
	}
	out := make([]joinrequests.Request, 0, len(raw))
	for key, value := range raw {
		var r joinrequests.Request
		if err := json.Unmarshal(value, &r); err != nil {
			return nil, fmt.Errorf("decode join request %s: %w", key, err)
		}
		out = append(out, r)
	}
	return out, nil
}

func (d *Domain) StoreJoinRequest(ctx context.Context, chatID int64, r joinrequests.Request) error {
	value, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("encode join request: %w", err)
	}
	if err := d.redis.SetHashValue(ctx, d.hashName(chatID), strconv.FormatInt(r.UserID, 10), value, 0); err != nil {
		return fmt.Errorf("write join request: %w", err)
	}
	return nil
}

func (d *Domain) DropJoinRequest(ctx context.Context, chatID, userID int64) error {
	if err := d.redis.DropHashValue(ctx, d.hashName(chatID), strconv.FormatInt(userID, 10)); err != nil {
		return fmt.Errorf("drop join request: %w", err)
	}
	return nil
}
