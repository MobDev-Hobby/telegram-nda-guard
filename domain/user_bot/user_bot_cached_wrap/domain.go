package user_bot_cached_wrap

import (
	"time"

	"go.uber.org/zap"
)

type Domain struct {
	userBotProvider UserBotProvider
	cacheTime       time.Duration
	cache           Cache
	log             Logger
}

func New(
	userBotProvider UserBotProvider,
	cacheTime time.Duration,
	log Logger,
) *Domain {
	logger := Logger(zap.NewNop().Sugar())
	if log != nil {
		logger = log
	}
	return &Domain{
		log:             logger,
		userBotProvider: userBotProvider,
		cacheTime:       cacheTime,
		cache: Cache{
			channelUsersCache: make(map[int64]ChannelUsersCache),
		},
	}
}
