package scanner

import (
	"reflect"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/MobDev-Hobby/telegram-nda-guard/processors"
	"github.com/MobDev-Hobby/telegram-nda-guard/storage/joinrequests"
	"github.com/MobDev-Hobby/telegram-nda-guard/storage/knownchats"
	"github.com/MobDev-Hobby/telegram-nda-guard/storage/whitelist"
)

type Domain struct {
	telegramBot TelegramBot
	userBot     UserBot
	log         Logger
	storage     ProtectedChannelStorage
	authorizer  Authorizer

	adminUserChatID        int64
	setAdminHash           *string
	withCommonLaunchNotify bool

	processingThreads int

	commandChannels          map[int64][]int64
	protectedChannels        map[int64]ProtectedChannel
	channels                 map[int64]ChannelInfo
	addChannelHandlers       map[int]int64
	addChannelRequestCounter int32

	// channelsMutex guards commandChannels, protectedChannels, channels and
	// addChannelHandlers. All four are read/written concurrently from the
	// access-rights checker, the scan worker pool, the ticker goroutine and
	// the per-update Telegram handlers (e.g. /add, /settings toggles, /remove).
	channelsMutex sync.RWMutex

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

	// Mini App support: manual kicks go through userKicker, scans are kept
	// in memory so a kick can only target users a scan actually returned.
	userKicker          UserKicker
	defaultCleanOptions processors.CleanOptions
	miniAppURL          string
	miniAppShortName    string
	scansMutex          sync.Mutex
	scans               map[string]*scanJob

	whitelistStorage      WhitelistStorage
	whitelistTTL          time.Duration
	whitelistRemindBefore time.Duration
	whitelistMutex        sync.Mutex
	whitelists            map[int64]map[int64]whitelist.Entry
	auditStorage          AuditStorage
	namesMutex            sync.Mutex
	userNames             map[int64]string

	knownChatStorage KnownChatStorage
	knownMutex       sync.Mutex
	knownChats       map[int64]knownchats.Chat

	joinRequestStorage JoinRequestStorage
	joinMutex          sync.Mutex
	joinRequests       map[int64]map[int64]joinrequests.Request

	// now is replaceable in tests.
	now func() time.Time

	// ready is closed once Run() finishes initialization, so HTTP/management
	// surfaces can wait for (or poll) readiness before serving traffic.
	ready      chan struct{}
	readyClose sync.Once
}

// Ready returns a channel that is closed when the controller has finished its
// initialization (bots started, commands registered, loops launched). HTTP
// surfaces can block on it or answer 503 until then.
func (d *Domain) Ready() <-chan struct{} {
	if d.ready == nil {
		ch := make(chan struct{})
		close(ch)
		return ch
	}
	return d.ready
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
		ready:              make(chan struct{}),
		scans:              make(map[string]*scanJob),

		whitelistTTL:          DefaultWhitelistTTL,
		whitelistRemindBefore: DefaultWhitelistRemindBefore,
		whitelists:            make(map[int64]map[int64]whitelist.Entry),
		now:                   time.Now,
		userNames:             make(map[int64]string),
		knownChats:            make(map[int64]knownchats.Chat),
		joinRequests:          make(map[int64]map[int64]joinrequests.Request),
	}

	for _, opt := range opts {
		opt(d)
	}

	return d
}
