package kicker

import (
	"context"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type Logger interface {
	Panicf(template string, args ...any)
	Errorf(template string, args ...any)
	Warnf(template string, args ...any)
	Infof(template string, args ...any)
	Debugf(template string, args ...any)
}

type TelegramBotUserKicker interface {
	SendMessage(
		ctx context.Context,
		params *bot.SendMessageParams,
	) (*models.Message, error)
	BanChatMember(
		ctx context.Context,
		params *bot.BanChatMemberParams,
	) (bool, error)
	UnbanChatMember(
		ctx context.Context,
		params *bot.UnbanChatMemberParams,
	) (bool, error)
}
