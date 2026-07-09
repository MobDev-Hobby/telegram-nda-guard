package main

import (
	"context"
	"crypto/aes"
	"crypto/tls"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
	goredis "github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	cachedchecker "github.com/MobDev-Hobby/telegram-nda-guard/checker/cached"
	demochecker "github.com/MobDev-Hobby/telegram-nda-guard/checker/demo"
	"github.com/MobDev-Hobby/telegram-nda-guard/controllers/scanner"
	"github.com/MobDev-Hobby/telegram-nda-guard/controllers/scanner/authorizer"
	"github.com/MobDev-Hobby/telegram-nda-guard/controllers/scanner/webapi"
	"github.com/MobDev-Hobby/telegram-nda-guard/processors/kicker"
	"github.com/MobDev-Hobby/telegram-nda-guard/processors/reporter"
	redischanstorage "github.com/MobDev-Hobby/telegram-nda-guard/storage/channels/redis"
	goredisadapter "github.com/MobDev-Hobby/telegram-nda-guard/storage/drivers/go-redis"
	filestorage "github.com/MobDev-Hobby/telegram-nda-guard/storage/session/file"
	redisstorage "github.com/MobDev-Hobby/telegram-nda-guard/storage/session/redis"
	"github.com/MobDev-Hobby/telegram-nda-guard/telegram/bots/bot"
	"github.com/MobDev-Hobby/telegram-nda-guard/telegram/sender/ratelimited"
	cacheduserbot "github.com/MobDev-Hobby/telegram-nda-guard/telegram/userbots/cached"
	"github.com/MobDev-Hobby/telegram-nda-guard/telegram/userbots/userbot"
)

// //nolint: revive
func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	loggerRaw, err := zap.NewDevelopment()
	if err != nil {
		log.Panicf("Can't init logger: %s", err)
	}
	logger := loggerRaw.Sugar()

	defer func() {
		if panicErr := recover(); panicErr != nil {
			// SugaredLogger.Error takes variadic args, not a context;
			// passing ctx logged a noisy struct dump instead of the panic.
			logger.Error(panicErr)
		}
	}()

	err = godotenv.Load()
	if err != nil {
		logger.Panicf("can't load .env: %s", err)
	}

	var options environmentConfig
	err = env.Parse(&options)
	if err != nil {
		logger.Panicf("can't parse env options: %s", err)
	}

	// Validate the AES key length explicitly: AES only accepts 16/24/32-byte
	// keys. aes.NewCipher would reject other lengths, but a clear message
	// helps operators diagnose a misconfigured SESSION_KEY.
	sessionKeyLen := len(options.SessionKey)
	if sessionKeyLen != 16 && sessionKeyLen != 24 && sessionKeyLen != 32 {
		logger.Panicf(
			"invalid SESSION_KEY length %d: AES key must be 16, 24 or 32 bytes",
			sessionKeyLen,
		)
	}
	cryptoProvider, err := aes.NewCipher(
		[]byte(options.SessionKey),
	)
	if err != nil {
		logger.Panicf("can't init crypto provider: %s", err)
	}

	var redisClient *goredisadapter.Domain
	if options.RedisHost != "" {
		redisOpts := &goredis.Options{
			Addr: options.RedisHost,
		}
		// AUTH: empty credentials keep the legacy no-auth behavior; setting
		// Password (and optionally Username for Redis 6 ACL) authenticates.
		if options.RedisPassword != "" {
			redisOpts.Username = options.RedisUsername
			redisOpts.Password = options.RedisPassword
		}
		// TLS: recommended for any non-loopback host so that credentials and
		// the session ciphertext are not sent in cleartext over the network.
		if options.RedisTLS {
			redisOpts.TLSConfig = &tls.Config{
				MinVersion: tls.VersionTLS12,
			}
		}
		redisConnect := goredis.NewClient(redisOpts)
		if redisConnect == nil {
			logger.Panicf("can't init redis: %s", err)
		}
		redisClient = goredisadapter.New(redisConnect)
	}

	var sessionStorageDomain userbot.SessionStorage
	if options.UseRedisSessionStorage {
		if redisClient == nil {
			logger.Panicf("can't init redis: %s", err)
		}

		sessionStorageDomain, err = redisstorage.New(
			cryptoProvider,
			redisClient,
			redisstorage.WithLogger(logger.Named("storage")),
		)
		if err != nil {
			logger.Panicf("can't init session storage: %s", err)
		}
	} else {
		fileOpts := []filestorage.SSOpts{
			filestorage.WithLogger(logger.Named("storage")),
		}
		// Default moved out of world-traversable /tmp; operators can still
		// override via SESSION_STORAGE_PATH.
		fileOpts = append(fileOpts, filestorage.WithStoragePath(defaultSessionStoragePath(options.SessionStoragePath)))
		sessionStorageDomain, err = filestorage.New(
			cryptoProvider,
			fileOpts...,
		)
		if err != nil {
			logger.Panicf("can't init session storage: %s", err)
		}
	}

	userBotDomain := userbot.New(
		options.TelegramAppID,
		options.TelegramAppKey,
		options.TelegramBotKey,
		sessionStorageDomain,
		userbot.WithLogger(logger.Named("user-bot")),
	)

	cachedUserBotDomain := cacheduserbot.New(
		userBotDomain,
		cacheduserbot.WithLogger(logger.Named("cached-user-bot")),
		cacheduserbot.WithCacheTTL(options.ChannelMembersCacheTTL),
	)

	telegramBotDomain, err := bot.New(
		options.TelegramBotKey,
		bot.WithLogger(logger.Named("telegram-bot")),
	)
	if err != nil {
		logger.Panicf("can't init session storage: %s", err)
	}

	telegramBotMessageSender := ratelimited.New(
		telegramBotDomain.GetBot(),
		ratelimited.WithLogger(logger.Named("telegram-bot-message-sender")),
	)

	accessChecker := demochecker.New()
	cachedAccessChecker := cachedchecker.New(
		accessChecker,
		cachedchecker.WithTTL(options.AccessCheckerCacheTTL),
	)

	commandChannels := make(map[int64][]int64)
	for channel, commandChannel := range options.CommandChannels {
		commandChannels[channel] = []int64{commandChannel}
	}

	scanReporter := reporter.New(
		telegramBotMessageSender,
		reporter.WithLogger(logger.Named("scan-report-processor")),
	)

	// The kicker must go through a rate-limited, FLOOD_WAIT-aware restrictor.
	// Previously it held the raw *bot.Bot and hit BanChatMember/UnbanChatMember
	// with no throttling — the most reliable way to get the account banned.
	kickerRestrictor := ratelimited.NewRestrictor(
		telegramBotDomain,
		1*time.Second,
		5,
		ratelimited.WithRestrictorLogger(logger.Named("kicker-restrictor")),
	)
	cleanReporter := kicker.New(
		kickerRestrictor,
		kicker.WithLogger(logger.Named("clean-report-processor")),
		kicker.WithCleanMessages(options.HideMessagesForKickedUsers),
		kicker.WithKeepBanned(options.KeepKickedUsersBanned),
		kicker.WithCleanUnknown(options.KickUnknownUsers),
	)

	channels := make([]scanner.ProtectedChannel, 0, len(options.Channels))
	for _, channelID := range options.Channels {
		channels = append(
			channels,
			scanner.ProtectedChannel{
				ID:                   channelID,
				CommandChannelIDs:    commandChannels[channelID],
				AutoScan:             true,
				AutoClean:            options.AutoKickUsers,
				AccessChecker:        cachedAccessChecker,
				ScanReportProcessor:  scanReporter,
				CleanReportProcessor: cleanReporter,
			},
		)
	}

	controllerOptions := []scanner.ProcessorOption{
		scanner.WithLogger(logger),
		scanner.WithOwnerChatID(options.AdminChatID),
		scanner.WithSetAdminKey(options.AdminSecret),
		scanner.WithChannelAutoScanInterval(options.ChannelScanInterval),
		scanner.WithCheckAccessInterval(options.AccessCheckInterval),
		scanner.WithChannels(channels),
		scanner.WithDefaultScanProcessor(scanReporter),
		scanner.WithDefaultCleanProcessor(cleanReporter),
		scanner.WithDefaultAccessChecker(cachedAccessChecker),
	}

	// Authorization: when requested, restrict commands to the owner and to the
	// administrators of the controlling chat. Otherwise commands stay open to
	// all members (backwards-compatible default).
	if options.RequireAdminAuth {
		authorizerOpts := []authorizer.Option{
			authorizer.WithOwner(options.AdminChatID),
			authorizer.WithRequireAdmin(),
			authorizer.WithLogger(logger.Named("authorizer")),
		}
		controllerOptions = append(
			controllerOptions,
			scanner.WithAuthorizer(authorizer.New(telegramBotDomain, authorizerOpts...)),
		)
		logger.Infof("Command authorization enabled (owner + chat admins)")
	}

	if redisClient != nil {
		storage, err := redischanstorage.New(redisClient)
		if err != nil {
			logger.Panicf("can't init storage: %s", err)
		}
		controllerOptions = append(controllerOptions, scanner.WithStorage(storage))
	}

	ProtectorControllerDomain := scanner.New(
		telegramBotDomain,
		cachedUserBotDomain,
		controllerOptions...,
	)

	// Optional web management dashboard. The same HybridAuthorizer serves both
	// the Telegram Authorizer and the WebAuthenticator roles, so web users are
	// authorized against the same owner/allowlist/admin policy.
	if options.WebAddr != "" {
		webAuth := authorizer.New(telegramBotDomain,
			authorizer.WithOwner(options.AdminChatID),
			authorizer.WithLogger(logger.Named("web-authorizer")),
		)
		webSecret := []byte(options.WebSessionSecret)
		if len(webSecret) < 32 {
			logger.Panicf("WEB_SESSION_SECRET must be at least 32 bytes when WEB_ADDR is set")
		}
		webServer, err := webapi.New(
			ProtectorControllerDomain,
			webAuth,
			options.TelegramBotKey,
			webSecret,
			webapi.WithLogger(logger.Named("webapi")),
		)
		if err != nil {
			logger.Panicf("can't init web api: %s", err)
		}
		// Wait for the controller to finish initializing before serving, then
		// run until the context is cancelled.
		go func() {
			<-ProtectorControllerDomain.Ready()
			logger.Infof("Web dashboard starting on %s", options.WebAddr)
			if err := webServer.ListenAndServe(options.WebAddr); err != nil {
				logger.Errorf("web api stopped: %s", err)
			}
		}()
	}

	err = ProtectorControllerDomain.Run(ctx)
	if err != nil {
		logger.Panicf("can't run bot: %s", err)
	}

	<-ctx.Done()

	logger.Infof("good bye!")
}

// defaultSessionStoragePath resolves the file-session directory. It prefers an
// explicit SESSION_STORAGE_PATH; otherwise it falls back to a local "data"
// directory (created with mode 0700) rather than the world-traversable /tmp,
// since the session blob is the auth key used to impersonate the bot.
func defaultSessionStoragePath(override string) string {
	if override != "" {
		return override
	}
	return "./data"
}
