package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/MobDev-Hobby/telegram-nda-guard/storage/channels"
)

func (s *Domain) Store(ctx context.Context, protectedChannel *channels.ProtectedChannel) error {

	channel, err := json.Marshal(protectedChannel)
	if err != nil {
		return fmt.Errorf("json marshal error: %w", err)
	}

	err = s.redis.SetHashValue(ctx, s.getHashName(), strconv.FormatInt(protectedChannel.ID, 10), channel, 0)
	if err != nil {
		return fmt.Errorf("write redis error: %w", err)
	}

	return nil
}
