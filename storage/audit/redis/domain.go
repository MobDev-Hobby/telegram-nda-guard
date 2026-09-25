// Package redis keeps channel action logs in capped Redis lists, newest first.
package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/MobDev-Hobby/telegram-nda-guard/storage/audit"
)

// RedisClient is the list subset of Redis the log needs.
type RedisClient interface {
	IsNil(err error) bool
	// ListPush prepends value to the list at key and trims it to maxLen.
	ListPush(ctx context.Context, key string, value []byte, maxLen int64) error
	// ListRange returns up to limit items from the head of the list at key.
	ListRange(ctx context.Context, key string, limit int64) ([][]byte, error)
}

type Domain struct {
	redis     RedisClient
	keyPrefix string
	maxLen    int64
}

type Option func(*Domain)

// WithMaxLen caps how many events are kept per channel (default 500).
func WithMaxLen(n int64) Option {
	return func(d *Domain) { d.maxLen = n }
}

// WithKeyPrefix overrides the list name prefix (default "pAudit").
func WithKeyPrefix(prefix string) Option {
	return func(d *Domain) { d.keyPrefix = prefix }
}

func New(redis RedisClient, opts ...Option) *Domain {
	d := &Domain{redis: redis, keyPrefix: "pAudit", maxLen: 500}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

func (d *Domain) key(channelID int64) string {
	return d.keyPrefix + ":" + strconv.FormatInt(channelID, 10)
}

// AppendAudit records an event for channelID.
func (d *Domain) AppendAudit(ctx context.Context, channelID int64, event audit.Event) error {
	value, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("encode audit event: %w", err)
	}
	if err := d.redis.ListPush(ctx, d.key(channelID), value, d.maxLen); err != nil {
		return fmt.Errorf("write audit event: %w", err)
	}
	return nil
}

// ListAudit returns up to limit most recent events of channelID, newest first.
func (d *Domain) ListAudit(ctx context.Context, channelID int64, limit int) ([]audit.Event, error) {
	raw, err := d.redis.ListRange(ctx, d.key(channelID), int64(limit))
	if err != nil {
		if d.redis.IsNil(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read audit log: %w", err)
	}
	events := make([]audit.Event, 0, len(raw))
	for _, item := range raw {
		var e audit.Event
		if err := json.Unmarshal(item, &e); err != nil {
			// One corrupt record must not hide the whole log.
			continue
		}
		events = append(events, e)
	}
	return events, nil
}
