package main

import (
	"context"
	"crypto/aes"
	"log"
	"os"
	"os/signal"

	"github.com/MobDev-Hobby/telegram-nda-guard/domain/access_checker/access_checker_cached_wrap"
	"github.com/MobDev-Hobby/telegram-nda-guard/domain/access_checker/access_checker_demo"
	"github.com/MobDev-Hobby/telegram-nda-guard/domain/processor/processor_access_control_demo"
	"github.com/MobDev-Hobby/telegram-nda-guard/domain/report_processor/report_processor_multiplexor"
	"github.com/MobDev-Hobby/telegram-nda-guard/domain/report_processor/report_processor_send_admin_with_telegram_bot"
	"github.com/MobDev-Hobby/telegram-nda-guard/domain/session_storage/session_storage_file"
	"github.com/MobDev-Hobby/telegram-nda-guard/domain/telegram_bot/telegram_bot"
	"github.com/MobDev-Hobby/telegram-nda-guard/domain/telegram_bot/telegram_bot_send_message_ratelimited"
	"github.com/MobDev-Hobby/telegram-nda-guard/domain/user_bot/user_bot"
	"github.com/MobDev-Hobby/telegram-nda-guard/domain/user_bot/user_bot_cached_wrap"
	"github.com/caarlos0/env"
	"github.com/joho/godotenv"
	"go.uber.org/zap"
)

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

	sessionStorageDomain, err := session_storage_file.New(
		"/tmp",
		cryptoProvider,
		logger,
	)
	if err != nil {
		logger.Panicf("can't init session storage: %s", err)
	}

	userBotDomain := user_bot.New(
		options.TelegramAppId,
		options.TelegramAppKey,
		logger,
	)

	cachedUserBotDomain := user_bot_cached_wrap.New(
		userBotDomain,
		options.ChannelMembersCacheTTL,
		logger,
	)

	telegramBotDomain, err := telegram_bot.New(
		options.TelegramBotKey,
		logger,
	)
	if err != nil {
		logger.Panicf("can't init session storage: %s", err)
	}
	
	telegramBotMessageSender := telegram_bot_send_message_ratelimited.New(
		telegramBotDomain.GetBot(),
		logger,
	)

	accessChecker := access_checker_demo.New()
	cachedAccessChecker := access_checker_cached_wrap.New(
		accessChecker,
		options.AccessCheckerCacheTTL,
	)

	channels := make(map[int64]processor_access_control_demo.CheckUserAccess)
	for _, channel := range options.Channels {
		channels[channel] = cachedAccessChecker
	}
	
	reportProcessorDomain := report_processor_multiplexor.New(
		logger,
		report_processor_send_admin_with_telegram_bot.New(
			telegramBotMessageSender,
			options.ReportChannels,
			logger,
		),
	)

	exampleProcessorDomain := processor_access_control_demo.New(
		sessionStorageDomain,
		telegramBotDomain,
		cachedUserBotDomain,
		reportProcessorDomain,
		options.AdminChatId,
		options.AdminSecret,
		logger,
		channels,
		options.AccessCheckInterval,
	)

	err = exampleProcessorDomain.Run(ctx)
	if err != nil {
		logger.Panicf("can't run bot: %s", err)
	}

	<-ctx.Done()

	logger.Infof("good bye!")
}
