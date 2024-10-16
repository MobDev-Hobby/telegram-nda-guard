package reporter

import (
	"go.uber.org/zap"
)

type Domain struct {
	log       Logger
	botClient TelegramBotMessageSender
}

func New(
	botClient TelegramBotMessageSender,
	opts ...Option,
) *Domain {

	d := &Domain{
		log:       Logger(zap.NewNop().Sugar()),
		botClient: botClient,
	}

	for _, opt := range opts {
		opt(d)
	}

	return d
}
