package report_processor_send_admin_with_telegram_bot

import (
	"github.com/MobDev-Hobby/telegram-nda-guard/interfaces"
	"github.com/go-telegram/bot"
)

type Logger interfaces.Logger

type TelegramBotProvider interface {
	GetBot() *bot.Bot
}
