package bot

import "github.com/go-telegram/bot"

func (d *Domain) GetBot() *bot.Bot {
	return d.botClient
}

func (d *Domain) UserID() int64 {
	return d.me.ID
}

func (d *Domain) Username() string {
	return d.me.Username
}
