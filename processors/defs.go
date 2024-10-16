package processors

import guard "github.com/MobDev-Hobby/telegram-nda-guard"

type AccessReport struct {
	Channel        guard.ChannelInfo
	ReportChannels []int64
	AllowedUsers   []guard.User
	DeniedUsers    []guard.User
	UnknownUsers   []guard.User
}
