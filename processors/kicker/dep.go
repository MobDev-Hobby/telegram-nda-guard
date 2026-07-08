package kicker

import (
	"context"
)

type Logger interface {
	Panicf(template string, args ...any)
	Errorf(template string, args ...any)
	Warnf(template string, args ...any)
	Infof(template string, args ...any)
	Debugf(template string, args ...any)
}

// UserRestrictor is the domain-level contract the kicker uses to act on users.
// It is intentionally transport-agnostic: implementations translate
// channelID/userID into the concrete Telegram API calls (including any
// chat-ID normalization such as the Bot API "-100" prefix).
//
// channelID is the same identifier that flows through AccessReport.Channel.ID
// (the bot-API-style channel identifier). Implementations are responsible for
// turning it into whatever form their transport expects.
type UserRestrictor interface {
	// SendReportMessage delivers a plain HTML text message (e.g. a cleanup
	// report) to the given chat. It is intentionally separate from a
	// full-featured SendMessage so processors do not need to construct a full
	// message DTO just to post a report.
	SendReportMessage(ctx context.Context, chatID int64, text string) error
	// Ban restricts userID in channelID. When revokeMessages is true, the
	// user's recent messages are deleted as part of the restriction.
	Ban(ctx context.Context, channelID, userID int64, revokeMessages bool) error
	// Unban lifts a previous restriction on userID in channelID.
	Unban(ctx context.Context, channelID, userID int64) error
}

// Compile-time guard: UserRestrictor is a domain interface independent of any
// concrete transport. Implementations live in the telegram/ packages.
var _ UserRestrictor = (UserRestrictor)(nil)

