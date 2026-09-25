// Package redis persists channel whitelists in Redis: one hash per channel,
// keyed by user ID.
package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/MobDev-Hobby/telegram-nda-guard/storage/whitelist"
)

// RedisClient is the subset of Redis operations the storage needs. It matches
// storage/channels/redis.RedisClient, so the same adapter serves both.
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

type Option func(d *Domain)

// WithKeyPrefix overrides the hash name prefix (default "pWhitelist").
func WithKeyPrefix(prefix string) Option {
	return func(d *Domain) {
		d.keyPrefix = prefix
	}
}

func New(redis RedisClient, opts ...Option) *Domain {
	d := &Domain{redis: redis, keyPrefix: "pWhitelist"}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

func (d *Domain) hashName(channelID int64) string {
	return d.keyPrefix + ":" + strconv.FormatInt(channelID, 10)
}

// LoadWhitelist returns every entry of channelID, expired ones included.
func (d *Domain) LoadWhitelist(ctx context.Context, channelID int64) ([]whitelist.Entry, error) {
	raw, err := d.redis.GetAllHashValues(ctx, d.hashName(channelID))
	if err != nil {
		if d.redis.IsNil(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read whitelist: %w", err)
	}
	entries := make([]whitelist.Entry, 0, len(raw))
	for key, value := range raw {
		var entry whitelist.Entry
		if err := json.Unmarshal(value, &entry); err != nil {
			return nil, fmt.Errorf("decode whitelist entry %s: %w", key, err)
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// StoreWhitelistEntry creates or replaces an entry.
func (d *Domain) StoreWhitelistEntry(ctx context.Context, channelID int64, entry whitelist.Entry) error {
	value, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("encode whitelist entry: %w", err)
	}
	if err := d.redis.SetHashValue(ctx, d.hashName(channelID), strconv.FormatInt(entry.UserID, 10), value, 0); err != nil {
		return fmt.Errorf("write whitelist entry: %w", err)
	}
	return nil
}

// DropWhitelistEntry removes an entry; removing a missing entry is not an
// error.
func (d *Domain) DropWhitelistEntry(ctx context.Context, channelID, userID int64) error {
	if err := d.redis.DropHashValue(ctx, d.hashName(channelID), strconv.FormatInt(userID, 10)); err != nil {
		return fmt.Errorf("drop whitelist entry: %w", err)
	}
	return nil
}
