package report_processor_send_admin_with_telegram_bot

import (
	"go.uber.org/zap"
)

type Domain struct {
	log           Logger
	botClient     TelegramBotMessageSender
	reportChatIds []int64
}

func New(
	botClient TelegramBotMessageSender,
	reportChatIds []int64,
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
	}
}
