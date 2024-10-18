package main

import (
	"context"
	"crypto/aes"
	"log"
	"os"
	"os/signal"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
	goredis "github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	cachedchecker "github.com/MobDev-Hobby/telegram-nda-guard/checker/cached"
	demochecker "github.com/MobDev-Hobby/telegram-nda-guard/checker/demo"
	"github.com/MobDev-Hobby/telegram-nda-guard/controllers/scanner"
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
			logger.Error(ctx, panicErr)
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

	cryptoProvider, err := aes.NewCipher(
		[]byte(options.SessionKey),
	)
	if err != nil {
		logger.Panicf("can't init crypto provider: %s", err)
	}

	var redisClient *goredisadapter.Domain
	if options.RedisHost != "" {
		redisConnect := goredis.NewClient(
			&goredis.Options{
				Addr: options.RedisHost,
			},
		)
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
		sessionStorageDomain, err = filestorage.New(
			cryptoProvider,
			filestorage.WithLogger(logger.Named("storage")),
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

	cleanReporter := kicker.New(
		telegramBotDomain.GetBot(),
		kicker.WithLogger(logger.Named("clean-report-processor")),
		kicker.WithCleanMessages(options.HideMessagesForKickedUsers),
		kicker.WithKeepBanned(options.KeepKickedUsersBanned),
		kicker.WithKeepBanned(options.KickUnknownUsers),
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

	err = ProtectorControllerDomain.Run(ctx)
	if err != nil {
		logger.Panicf("can't run bot: %s", err)
	}

	<-ctx.Done()

	logger.Infof("good bye!")
}
