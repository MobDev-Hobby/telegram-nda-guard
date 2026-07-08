package processors

import guard "github.com/MobDev-Hobby/telegram-nda-guard"

type AccessReport struct {
	Channel        guard.ChannelInfo
	ReportChannels []int64
	AllowedUsers   []guard.User
	DeniedUsers    []guard.User
	UnknownUsers   []guard.User
	// CleanOptions optionally carries per-channel cleanup behavior for the
	// cleaner processor (kicker). When nil, the processor falls back to its own
	// configured defaults. Scan-only processors ignore this field.
	CleanOptions *CleanOptions
}

// CleanOptions describes how a channel should be cleaned. It is read by the
// kicker processor and can be supplied per-channel via AccessReport so that
// different channels get different cleanup behavior (e.g. one channel keeps
// users banned, another only kicks).
type CleanOptions struct {
	// KeepBanned, when true, leaves banned users banned (no unban follow-up).
	// When false, the user is unbanned immediately after the ban, i.e. kicked
	// but able to rejoin.
	KeepBanned bool
	// CleanMessages, when true, revokes/deletes the kicked user's recent
	// messages. Only meaningful together with a ban.
	CleanMessages bool
	// CleanUnknown, when true, also kicks users whose access could not be
	// determined (in addition to denied users).
	CleanUnknown bool
}
