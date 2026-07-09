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
// implementation (see telegram/sender/ratelimited.Restrictor). Hitting the
// ban/unban endpoints on the raw *bot.Bot without throttling is the most
// reliable way to get the bot account restricted or banned.
type TelegramBotUserKicker interface {
	Ban(ctx context.Context, channelID, userID int64, revokeMessages bool) error
	Unban(ctx context.Context, channelID, userID int64, onlyIfBanned bool) error
	SendReport(ctx context.Context, chatID int64, text string) error
}
