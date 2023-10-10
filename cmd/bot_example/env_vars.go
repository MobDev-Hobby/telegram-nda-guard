package main

import "time"

type environmentConfig struct {
	TelegramBotKey             string        `env:"TELEGRAM_BOT_KEY"`
	TelegramAppId              int           `env:"TELEGRAM_APP_ID"`
	TelegramAppKey             string        `env:"TELEGRAM_APP_KEY"`
	SessionKey                 string        `env:"SESSION_KEY"`
	AdminChatId                int64         `env:"ADMIN_CHAT_ID"`
	AdminSecret                string        `env:"ADMIN_SECRET"`
	Channels                   []int64       `env:"CHANNELS" envSeparator:","`
	AccessCheckInterval        time.Duration `env:"ACCESS_CHECK_INTERVAL"`
	ChannelMembersCacheTTL     time.Duration `env:"CHANNEL_MEMBERS_CACHE_TTL"`
	AccessCheckerCacheTTL      time.Duration `env:"ACCESS_CHECKER_CACHE_TTL"`
	ReportChannels             []int64       `env:"REPORT_CHANNELS" envSeparator:","`
	AutoKickUsers              bool          `env:"AUTO_KICK_USERS"`
	KeepKickedUsersBanned      bool          `env:"KEEP_KICKED_USERS_BANNED"`
	HideMessagesForKickedUsers bool          `env:"HIDE_MESSAGES_FOR_KICKED_USERS"`
	KickUnknownUsers           bool          `env:"KICK_UNKNOWN_USERS"`
}
