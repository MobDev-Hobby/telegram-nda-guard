package redis

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/MobDev-Hobby/telegram-nda-guard/storage/channels"
)

func (s *Domain) LoadAll(ctx context.Context) ([]channels.ProtectedChannel, error) {

	s.log.Debugf("try to load channels from redis: %s", s.getHashName())

	result, err := s.redis.GetAllHashValues(ctx, s.getHashName())
	if err != nil {
		if s.redis.IsNil(err) {
			s.log.Debugf("data not found in redis: %s", s.getHashName())
			return nil, nil
		}
		s.log.Errorf("redis read error: %s, %s", s.getHashName(), err)
		return nil, fmt.Errorf("read redis error: %w", err)
	}

	protectedChannels := make([]channels.ProtectedChannel, 0, len(result))
	for _, protectedChannelData := range result {
		var protectedChannel channels.ProtectedChannel
		err = json.Unmarshal(protectedChannelData, &protectedChannel)
		if err != nil {
			s.log.Errorf("json unmarshal error: %s, %s", protectedChannelData, err)
			return nil, fmt.Errorf("json unmarshal error: %w", err)
		}
		protectedChannels = append(protectedChannels, protectedChannel)
	}

	return protectedChannels, nil
}
