// Package redis keeps known chats in one Redis hash keyed by chat ID.
package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/MobDev-Hobby/telegram-nda-guard/storage/knownchats"
)

// RedisClient matches storage/channels/redis.RedisClient.
type RedisClient interface {
	IsNil(err error) bool
	DropHashValue(ctx context.Context, hash string, key string) error
	SetHashValue(ctx context.Context, hash string, key string, value []byte, expiration time.Duration) error
	GetAllHashValues(ctx context.Context, hash string) (map[string][]byte, error)
}

type Domain struct {
	redis RedisClient
	hash  string
}

func New(redis RedisClient) *Domain {
	return &Domain{redis: redis, hash: "pKnownChats"}
}

func (d *Domain) LoadKnownChats(ctx context.Context) ([]knownchats.Chat, error) {
	raw, err := d.redis.GetAllHashValues(ctx, d.hash)
	if err != nil {
		if d.redis.IsNil(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read known chats: %w", err)
	}
	out := make([]knownchats.Chat, 0, len(raw))
	for key, value := range raw {
		var c knownchats.Chat
		if err := json.Unmarshal(value, &c); err != nil {
			return nil, fmt.Errorf("decode known chat %s: %w", key, err)
		}
		out = append(out, c)
	}
	return out, nil
}

func (d *Domain) StoreKnownChat(ctx context.Context, chat knownchats.Chat) error {
	value, err := json.Marshal(chat)
	if err != nil {
		return fmt.Errorf("encode known chat: %w", err)
	}
	if err := d.redis.SetHashValue(ctx, d.hash, strconv.FormatInt(chat.ID, 10), value, 0); err != nil {
		return fmt.Errorf("write known chat: %w", err)
	}
	return nil
}

func (d *Domain) DropKnownChat(ctx context.Context, chatID int64) error {
	if err := d.redis.DropHashValue(ctx, d.hash, strconv.FormatInt(chatID, 10)); err != nil {
		return fmt.Errorf("drop known chat: %w", err)
	}
	return nil
}
