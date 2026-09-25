package processors

import guard "github.com/MobDev-Hobby/telegram-nda-guard"

// CleanOptions controls how a cleaner removes users from a channel.
type CleanOptions struct {
	// KeepBanned leaves removed users banned instead of just kicking them.
	KeepBanned bool `json:"keepBanned"`
	// CleanMessages deletes the removed users' messages.
	CleanMessages bool `json:"cleanMessages"`
	// CleanUnknown also removes users whose access could not be checked.
	CleanUnknown bool `json:"cleanUnknown"`
}

type AccessReport struct {
	Channel        guard.ChannelInfo
	ReportChannels []int64
	AllowedUsers   []guard.User
	DeniedUsers    []guard.User
	UnknownUsers   []guard.User
	// CleanOptions, when set, overrides the cleaner's process-wide defaults
	// for this channel.
	CleanOptions *CleanOptions
	// Stats tells how complete the member list was. Zero value = unknown.
	Stats guard.ScanStats
}

// KickResult is the outcome of removing one user.
type KickResult struct {
	UserID int64  `json:"userId"`
	OK     bool   `json:"ok"`
	Error  string `json:"error,omitempty"`
}
