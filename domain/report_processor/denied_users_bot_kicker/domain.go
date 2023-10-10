package denied_users_bot_kicker

import (
	"go.uber.org/zap"
)

type Domain struct {
	log           Logger
	botClient     TelegramBotUserKicker
	cleanMessages bool
	keepBanned    bool
	cleanUnknown  bool
	reportChatIds []int64
}

func New(
	botClient TelegramBotUserKicker,
	reportChatIds []int64,
	cleanMessages bool,
	keepBanned bool,
	cleanUnknown bool,
	log Logger,
) *Domain {
	logger := Logger(zap.NewNop().Sugar())
	if log != nil {
		logger = log
	}
	return &Domain{
		log:           logger,
		botClient:     botClient,
		reportChatIds: reportChatIds,
		keepBanned:    keepBanned,
		cleanMessages: cleanMessages,
		cleanUnknown:  cleanUnknown,
	}
}
