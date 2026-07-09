package kicker

import "context"

type Logger interface {
	Panicf(template string, args ...any)
	Errorf(template string, args ...any)
	Warnf(template string, args ...any)
	Infof(template string, args ...any)
	Debugf(template string, args ...any)
}

// TelegramBotUserKicker restricts and notifies users. It abstracts over the
// raw Bot API so the kicker can be backed by a rate-limited, FLOOD_WAIT-aware
// implementation (see telegram/sender/ratelimited). Hitting Telegram's ban /
// unban endpoints without throttling gets the bot account restricted.
type TelegramBotUserKicker interface {
	Ban(ctx context.Context, channelID, userID int64, revokeMessages bool) error
	Unban(ctx context.Context, channelID, userID int64) error
	SendReport(ctx context.Context, chatID int64, text string) error
}
