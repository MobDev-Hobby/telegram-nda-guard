package denied_users_bot_kicker

import (
	"context"

	"github.com/MobDev-Hobby/telegram-nda-guard/interfaces"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type Logger interfaces.Logger

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
