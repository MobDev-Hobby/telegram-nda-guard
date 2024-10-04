package userbot

import (
	"github.com/gotd/td/tg"
)

func (d *Domain) GetDispatcher() tg.UpdateDispatcher {
	return d.userBot.dispatcher
}
