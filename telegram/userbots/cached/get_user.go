package cached

func (d *Domain) UserID() int64 {
	return d.userBot.UserID()
}
func (d *Domain) Username() string {
	return d.userBot.Username()
}
