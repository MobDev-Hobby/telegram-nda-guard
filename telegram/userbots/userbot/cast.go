package userbot

func (d *UserBotInstance) botChannelIDtoUserChannelID(botChannelID int64) int64 {
	return -botChannelID - 1000000000000
}
