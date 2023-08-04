package cached_user_bot

import (
	"time"

	"go.uber.org/zap"
)

type Domain struct {
	userBotProvider UserBotProvider
	cacheTime       time.Duration
	cache           Cache
	log             *zap.SugaredLogger
}

func New(
	userBotProvider UserBotProvider,
	cacheTime time.Duration,
	log *zap.SugaredLogger,
) *Domain {
	logger := zap.NewNop().Sugar()
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
