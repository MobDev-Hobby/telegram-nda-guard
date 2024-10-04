package userbot

func (d *Domain) UserID() int64 {
	return d.userBot.me.ID
}

func (d *Domain) Username() string {
	return d.userBot.me.Username
}
