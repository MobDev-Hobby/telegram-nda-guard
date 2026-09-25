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
}
