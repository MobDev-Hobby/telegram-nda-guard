package scanner

import (
	"reflect"
	"sync"
	"time"

	"go.uber.org/zap"
)

type Domain struct {
	telegramBot TelegramBot
	userBot     UserBot
	log         Logger
	storage     ProtectedChannelStorage

	adminUserChatID        int64
	setAdminHash           *string
	withCommonLaunchNotify bool

	processingThreads int

	commandChannels    map[int64][]int64
	protectedChannels  map[int64]ProtectedChannel
	channels           map[int64]ChannelInfo
	addChannelHandlers map[int]int64

	accessCheckInterval     time.Duration
	channelAutoScanInterval time.Duration
	taskDelayInterval       time.Duration

	processRequestChan chan ScanRequest

	defaultScanProcessor  UserReportProcessor
	defaultCleanProcessor UserReportProcessor
	defaultAccessChecker  CheckUserAccess

	tickerCasesMutex    sync.Mutex
	tickerCases         []reflect.SelectCase
	tickerCasesChannels []int64
}

func New(
	telegramBot TelegramBot,
	userBot UserBot,
	opts ...ProcessorOption,
) *Domain {

	d := &Domain{
		telegramBot: telegramBot,
		userBot:     userBot,
		log:         Logger(zap.NewNop().Sugar()),

		adminUserChatID:   -1,
		setAdminHash:      nil,
		processingThreads: 4,

		accessCheckInterval:     10 * time.Minute,
		channelAutoScanInterval: 6 * time.Hour,
		taskDelayInterval:       10 * time.Second,

		commandChannels:    make(map[int64][]int64),
		protectedChannels:  make(map[int64]ProtectedChannel),
		channels:           make(map[int64]ChannelInfo),
		addChannelHandlers: make(map[int]int64),

		processRequestChan: make(chan ScanRequest, 10),
	}

	for _, opt := range opts {
		opt(d)
	}

	return d
}
