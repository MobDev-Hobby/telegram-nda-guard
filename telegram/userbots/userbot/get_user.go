package userbot

func (d *Domain) UserID() int64 {
	if d.userBot.me == nil {
		d.log.Errorf("Can't get user bot ID")
		return 0
	}
	return d.userBot.me.ID
}

func (d *Domain) Username() string {
	if d.userBot.me == nil {
		d.log.Errorf("Can't get user bot Username")
		return ""
	}
	return d.userBot.me.Username
}
