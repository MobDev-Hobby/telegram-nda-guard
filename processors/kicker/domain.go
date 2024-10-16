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
}

func New(
	botClient TelegramBotUserKicker,
	opts ...Option,
) *Domain {

	d := &Domain{
		log:           Logger(zap.NewNop().Sugar()),
		botClient:     botClient,
		keepBanned:    false,
		cleanMessages: true,
		cleanUnknown:  false,
	}

	for _, opt := range opts {
		opt(d)
	}

	return d
}
