package telegram_bot

import "github.com/go-telegram/bot"

func (d *Domain) GetBot() *bot.Bot {
	return d.botClient
}
