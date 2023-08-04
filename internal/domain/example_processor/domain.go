package example_processor

import (
	"time"

	"go.uber.org/zap"
)

const userBotName = "example_bot"

type Domain struct {
	sessionStorageProvider StorageProvider
	telegramBot            TelegramBotProvider
	userBotProvider        UserBotProvider
	userBot                UserBot
	adminUserChatId        int64
	setAdminHash           string
	log                    *zap.SugaredLogger
	accessCheckers         map[int64]CheckUserAccess
	accessCheckInterval    time.Duration
}

func New(
	sessionStorageProvider StorageProvider,
	telegramBot TelegramBotProvider,
	userBotProvider UserBotProvider,
	adminUserChatId int64,
	setAdminHash string,
	log *zap.SugaredLogger,
	accessCheckers map[int64]CheckUserAccess,
	accessCheckInterval time.Duration,
) *Domain {
	logger := zap.NewNop().Sugar()
	if log != nil {
		logger = log
	}
	return &Domain{
		sessionStorageProvider: sessionStorageProvider,
		telegramBot:            telegramBot,
		userBotProvider:        userBotProvider,
		adminUserChatId:        adminUserChatId,
		setAdminHash:           setAdminHash,
		log:                    logger,
		accessCheckers:         accessCheckers,
		accessCheckInterval:    accessCheckInterval,
	}
}
