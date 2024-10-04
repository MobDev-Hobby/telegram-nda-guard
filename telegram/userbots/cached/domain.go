package cached

import (
	"time"

	"go.uber.org/zap"
)

type Domain struct {
	userBot   UserBot
	cacheTime time.Duration
	cache     Cache
	log       Logger
}

func New(
	userBot UserBot,
	opts ...UserBotCachedOption,
) *Domain {

	d := &Domain{
		log:       Logger(zap.NewNop().Sugar()),
		userBot:   userBot,
		cacheTime: 30 * time.Minute,
		cache: Cache{
			channelUsersCache: make(map[int64]ChannelUsersCache),
		},
	}

	for _, opt := range opts {
		opt(d)
	}

	// Setup cache update hooks
	userBot.GetDispatcher().OnChatParticipantAdd(d.HandleChatParticipantAdd)
	userBot.GetDispatcher().OnChatParticipantDelete(d.HandleChatParticipantDelete)
	userBot.GetDispatcher().OnNewChannelMessage(d.HandleNewChannelMessage)

	return d
}
