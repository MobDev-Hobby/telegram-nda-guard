package channels

import "github.com/MobDev-Hobby/telegram-nda-guard/processors"

type ProtectedChannel struct {
	ID                int64
	CommandChannelIDs []int64
	AutoScan          bool
	AutoClean         bool
	AllowClean        bool
	// CleanOptions overrides the cleaner defaults for this channel. Absent in
	// records written by older versions, which then keep using the defaults.
	CleanOptions *processors.CleanOptions `json:",omitempty"`
	// Managers are the users who joined the channel in the bot (Mini App);
	// they manage it and get its reminders.
	Managers []int64 `json:",omitempty"`
	// LastCheck is the latest member check, for the health indicator.
	LastCheck *processors.CheckSummary `json:",omitempty"`
	// JoinRequestMode is off, auto or manual (empty = off).
	JoinRequestMode string `json:",omitempty"`
}
