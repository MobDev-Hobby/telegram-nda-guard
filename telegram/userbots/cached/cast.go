package cached

func (d *Domain) userChannelIDtoBotChannelID(userChannelID int64) int64 {
	return -(userChannelID + 1000000000000)
}
