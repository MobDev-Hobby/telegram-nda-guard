package main

import (
	"context"
	"crypto/aes"
	"log"
	"os"
	"os/signal"

	"github.com/Mobdev-Hobby/telegram-nda-guard/internal/domain/access_checker_demo"
	"github.com/Mobdev-Hobby/telegram-nda-guard/internal/domain/cached_user_bot"
	"github.com/Mobdev-Hobby/telegram-nda-guard/internal/domain/example_processor"
	"github.com/Mobdev-Hobby/telegram-nda-guard/internal/domain/session_storage"
	"github.com/Mobdev-Hobby/telegram-nda-guard/internal/domain/telegram_bot"
	"github.com/Mobdev-Hobby/telegram-nda-guard/internal/domain/user_bot"
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

	sessionStorageDomain, err := session_storage.New(
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
	
	cachedUserBotDomain := cached_user_bot.New(
		userBotDomain,
		options.ChannelMembersCacheTTL,
		logger,
	)

	telegramBotDomain := telegram_bot.New(
		options.TelegramBotKey,
		logger,
	)

	accessChecker := access_checker_demo.New()

	
	channels := make(map[int64]example_processor.CheckUserAccess)
	for _, channel := range options.Channels {
		channels[channel] = accessChecker
	}
	
	
	exampleProcessorDomain := example_processor.New(
		sessionStorageDomain,
		telegramBotDomain,
		cachedUserBotDomain,
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
	
	<- ctx.Done()

	logger.Infof("good bye!")
}