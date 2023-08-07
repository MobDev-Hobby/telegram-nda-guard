package processor_access_control_demo

import (
	"time"

	"go.uber.org/zap"
)

const userBotName = "example_bot"

type Domain struct {
	sessionStorageProvider StorageProvider
	telegramBot            TelegramBotProvider
	userBotProvider        UserBotProvider
	reportProcessor        UserReportProcessor
	userBot                UserBot
	adminUserChatId        int64
	setAdminHash           string
	log                    Logger
	accessCheckers         map[int64]CheckUserAccess
	accessCheckInterval    time.Duration
}

func New(
	sessionStorageProvider StorageProvider,
	telegramBot TelegramBotProvider,
	userBotProvider UserBotProvider,
	reportProcessor UserReportProcessor,
	adminUserChatId int64,
	setAdminHash string,
	log Logger,
	accessCheckers map[int64]CheckUserAccess,
	accessCheckInterval time.Duration,
) *Domain {
	logger := Logger(zap.NewNop().Sugar())
	if log != nil {
		logger = log
	}
	return &Domain{
		sessionStorageProvider: sessionStorageProvider,
		telegramBot:            telegramBot,
		userBotProvider:        userBotProvider,
		reportProcessor:        reportProcessor,
		adminUserChatId:        adminUserChatId,
		setAdminHash:           setAdminHash,
		log:                    logger,
		accessCheckers:         accessCheckers,
		accessCheckInterval:    accessCheckInterval,
	}
}
