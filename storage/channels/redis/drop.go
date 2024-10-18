package redis

import (
	"context"
	"fmt"
	"strconv"

	"github.com/MobDev-Hobby/telegram-nda-guard/controllers/scanner"
)

func (s *Domain) Drop(ctx context.Context, protectedChannel *scanner.ProtectedChannel) error {

	err := s.redis.DropHashValue(ctx, s.getHashName(), strconv.FormatInt(protectedChannel.ID, 10))
	if err != nil {
		return fmt.Errorf("write redis error: %w", err)
	}

	return nil
}
