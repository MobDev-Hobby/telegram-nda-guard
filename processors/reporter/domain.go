package reporter

import (
	"go.uber.org/zap"
)

type Domain struct {
	log           Logger
	botClient     TelegramBotMessageSender
	reportChatIDs map[int64][]int64
}

func New(
	botClient TelegramBotMessageSender,
	reportChatIDs map[int64][]int64, // channelID => reportChannelID
	opts ...Option,
) *Domain {

	d := &Domain{
		log:           Logger(zap.NewNop().Sugar()),
		botClient:     botClient,
		reportChatIDs: reportChatIDs,
	}

	for _, opt := range opts {
		opt(d)
	}

	return d
}
