package redis

import (
	"context"
	"fmt"
	"strconv"
)

// Drop removes the persisted record of the protected channel identified by
// channelID. It is idempotent: dropping a channel with no persisted record does
// not return an error (the underlying hash-key delete is a no-op in that case).
func (s *Domain) Drop(ctx context.Context, channelID int64) error {

	err := s.redis.DropHashValue(
		ctx,
		s.getHashName(),
		strconv.FormatInt(channelID, 10),
	)
	if err != nil {
		return fmt.Errorf("write redis error: %w", err)
	}

	return nil
}
