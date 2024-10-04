package processors

import guard "github.com/MobDev-Hobby/telegram-nda-guard"

type AccessReport struct {
	Channel      guard.ChannelInfo
	AllowedUsers []guard.User
	DeniedUsers  []guard.User
	UnknownUsers []guard.User
}
