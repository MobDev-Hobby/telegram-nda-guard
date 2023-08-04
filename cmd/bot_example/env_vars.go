package main

import "time"

type environmentConfig struct {
	TelegramBotKey         string        `env:"TELEGRAM_BOT_KEY"`
	TelegramAppId          int           `env:"TELEGRAM_APP_ID"`
	TelegramAppKey         string        `env:"TELEGRAM_APP_KEY"`
	SessionKey             string        `env:"SESSION_KEY"`
	AdminChatId            int64         `env:"ADMIN_CHAT_ID"`
	AdminSecret            string        `env:"ADMIN_SECRET"`
	Channels               []int64       `env:"CHANNELS" envSeparator:","`
	AccessCheckInterval    time.Duration `env:"ACCESS_CHECK_INTERVAL"`
	ChannelMembersCacheTTL time.Duration `env:"CHANNEL_MEMBERS_CACHE_TTL"`
}
