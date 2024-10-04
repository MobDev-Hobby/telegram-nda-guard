package main

import "time"

type environmentConfig struct {
	TelegramBotKey             string          `env:"TELEGRAM_BOT_KEY"`
	TelegramAppID              int             `env:"TELEGRAM_APP_ID"`
	TelegramAppKey             string          `env:"TELEGRAM_APP_KEY"`
	SessionKey                 string          `env:"SESSION_KEY"`
	AdminChatID                int64           `env:"ADMIN_CHAT_ID"`
	AdminSecret                string          `env:"ADMIN_SECRET"`
	Channels                   []int64         `env:"CHANNELS" envSeparator:","`
	CommandChannels            map[int64]int64 `env:"COMMAND_CHANNELS" envSeparator:"," envKeyValSeparator:":"`
	AccessCheckInterval        time.Duration   `env:"ACCESS_CHECK_INTERVAL"`
	ChannelScanInterval        time.Duration   `env:"CHANNEL_SCAN_INTERVAL"`
	ChannelMembersCacheTTL     time.Duration   `env:"CHANNEL_MEMBERS_CACHE_TTL"`
	AccessCheckerCacheTTL      time.Duration   `env:"ACCESS_CHECKER_CACHE_TTL"`
	AutoKickUsers              bool            `env:"AUTO_KICK_USERS"`
	KeepKickedUsersBanned      bool            `env:"KEEP_KICKED_USERS_BANNED"`
	HideMessagesForKickedUsers bool            `env:"HIDE_MESSAGES_FOR_KICKED_USERS"`
	KickUnknownUsers           bool            `env:"KICK_UNKNOWN_USERS"`
	UseRedisSessionStorage     bool            `env:"USE_REDIS_SESSION_STORAGE"`
	RedisHost                  string          `env:"REDIS_HOST"`
}
