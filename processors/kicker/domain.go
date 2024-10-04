package kicker

import (
	"go.uber.org/zap"
)

type Domain struct {
	log           Logger
	botClient     TelegramBotUserKicker
	cleanMessages bool
	keepBanned    bool
	cleanUnknown  bool
	reportChatIDs map[int64][]int64 // channel => []reportChannel
}

func New(
	botClient TelegramBotUserKicker,
	reportChatIDs map[int64][]int64,
	opts ...Option,
) *Domain {

	d := &Domain{
		log:           Logger(zap.NewNop().Sugar()),
		botClient:     botClient,
		reportChatIDs: reportChatIDs,
		keepBanned:    false,
		cleanMessages: true,
		cleanUnknown:  false,
	}

	for _, opt := range opts {
		opt(d)
	}

	return d
}
