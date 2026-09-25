package scanner

import (
	"time"

	"github.com/MobDev-Hobby/telegram-nda-guard/processors"
)

type ProcessorOption func(*Domain)

func WithLogger(log Logger) func(*Domain) {
	if log == nil {
		panic("log is nil")
	}
	return func(d *Domain) {
		d.log = log
	}
}

func WithSetAdminKey(key string) func(*Domain) {
	if len(key) < 32 {
		panic("insecure root key")
	}
	return func(d *Domain) {
		d.setAdminHash = &key
	}
}

func WithChannelAutoScanInterval(interval time.Duration) func(*Domain) {
	if interval < 10*time.Second {
		panic("too small auto scan interval can occur ban")
	}
	return func(d *Domain) {
		d.channelAutoScanInterval = interval
	}
}

func WithCheckAccessInterval(interval time.Duration) func(*Domain) {
	if interval < 10*time.Second {
		panic("too small auto scan interval can occur ban")
	}
	return func(d *Domain) {
		d.accessCheckInterval = interval
	}
}

func WithOwnerChatID(ownerChatID int64) func(*Domain) {
	if ownerChatID <= 0 {
		panic("invalid ownerChatID")
	}
	return func(d *Domain) {
		d.adminUserChatID = ownerChatID
	}
}

func WithChannels(channels []ProtectedChannel) func(*Domain) {
	return func(d *Domain) {
		for _, channel := range channels {
			channel := channel
			err := d.AddProtectedChannel(&channel)
			if err != nil {
				panic("cannot add channel to domain")
			}
		}
	}
}

func WithCustomScheduledChannel(channel ProtectedChannel, tickerChan chan time.Time) func(*Domain) {
	return func(d *Domain) {
		err := d.AddProtectedChannel(&channel, WithTickerChan(tickerChan))
		if err != nil {
			panic("cannot add channel to domain")
		}
	}
}

func WithCronScheduledChannel(channel ProtectedChannel, cronString string) func(*Domain) {
	return func(d *Domain) {
		err := d.AddProtectedChannel(&channel, WithCron(cronString))
		if err != nil {
			panic("cannot add channel to domain")
		}
	}
}

func WithTaskDelayInterval(interval time.Duration) func(*Domain) {
	return func(d *Domain) {
		d.taskDelayInterval = interval
	}
}

func WithNProcessingThreads(threads int) func(*Domain) {
	if threads <= 0 {
		panic("invalid threads quantity, bot will not work")
	}
	return func(d *Domain) {
		d.processingThreads = threads
	}
}

func WithDefaultScanProcessor(processor UserReportProcessor) func(*Domain) {
	return func(d *Domain) {
		d.defaultScanProcessor = processor
	}
}

func WithDefaultCleanProcessor(processor UserReportProcessor) func(*Domain) {
	return func(d *Domain) {
		d.defaultCleanProcessor = processor
	}
}

func WithDefaultAccessChecker(checker CheckUserAccess) func(*Domain) {
	return func(d *Domain) {
		d.defaultAccessChecker = checker
	}
}

func WithCommonLaunchNotify() func(*Domain) {
	return func(d *Domain) {
		d.withCommonLaunchNotify = true
	}
}

func WithStorage(storage ProtectedChannelStorage) func(*Domain) {
	return func(d *Domain) {
		d.storage = storage
	}
}

// WithAuthorizer injects a command authorization strategy. When not provided,
// the controller uses an allow-all authorizer (backwards-compatible with the
// pre-authorization behavior, where any member of a controlling chat could run
// commands). Passing a custom Authorizer (e.g. the bundled HybridAuthorizer)
// restricts who can run protected commands. See NewHybridAuthorizer.
func WithAuthorizer(authorizer Authorizer) func(*Domain) {
	if authorizer == nil {
		panic("authorizer is nil")
	}
	return func(d *Domain) {
		d.authorizer = authorizer
	}
}

// WithUserKicker enables manual kicks from the Mini App. The bundled
// processors/kicker.Domain implements UserKicker; wire the same instance that
// serves as the default clean processor so both paths share rate limiting.
func WithUserKicker(kicker UserKicker) func(*Domain) {
	return func(d *Domain) {
		d.userKicker = kicker
	}
}

// WithDefaultCleanOptions tells the controller which clean options apply to
// channels without their own. It is used to display settings; keep it equal
// to the defaults the clean processor was built with.
func WithDefaultCleanOptions(opts processors.CleanOptions) func(*Domain) {
	return func(d *Domain) {
		d.defaultCleanOptions = opts
	}
}

// WithMiniApp enables the /app command and the bot menu button. url is the
// HTTPS address of the Mini App page. shortName is the Mini App short name
// registered in BotFather; it is needed to open the app from groups, where
// Telegram only allows t.me/<bot>/<shortName> links.
func WithMiniApp(url, shortName string) func(*Domain) {
	return func(d *Domain) {
		d.miniAppURL = url
		d.miniAppShortName = shortName
	}
}

// WithWhitelistStorage enables per-channel whitelists: channel administrators
// approve users from the Mini App, approved users skip the access check until
// the approval expires (WithWhitelistTTL, 30 days by default) and must then be
// re-approved.
func WithWhitelistStorage(storage WhitelistStorage) func(*Domain) {
	return func(d *Domain) {
		d.whitelistStorage = storage
	}
}

// WithWhitelistTTL sets how long a whitelist approval lasts.
func WithWhitelistTTL(ttl time.Duration) func(*Domain) {
	if ttl <= 0 {
		panic("whitelist ttl must be positive")
	}
	return func(d *Domain) {
		d.whitelistTTL = ttl
	}
}

// WithWhitelistRemindBefore sets how early control chats are reminded that an
// approval is running out.
func WithWhitelistRemindBefore(before time.Duration) func(*Domain) {
	return func(d *Domain) {
		d.whitelistRemindBefore = before
	}
}
