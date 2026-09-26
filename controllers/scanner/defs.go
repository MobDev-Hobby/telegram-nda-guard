package scanner

import "github.com/MobDev-Hobby/telegram-nda-guard/processors"

type ChannelInfo struct {
	migratedFrom      *int64
	id                int64
	commandChannelIDs []int64
	title             string
	chatType          string
	botOnChannel      bool
	botCanInvite      bool
	botCanClean       bool
}

func (ci *ChannelInfo) CanScan() bool {
	return ci.botOnChannel
}

func (ci *ChannelInfo) CanClean() bool {
	return ci.botOnChannel && ci.botCanClean
}

type ProtectedChannel struct {
	ID                int64
	CommandChannelIDs []int64
	AutoScan          bool
	AutoClean         bool
	AllowClean        bool
	// CleanOptions overrides the clean processor's defaults for this channel.
	// nil means "use the defaults".
	CleanOptions *processors.CleanOptions
	// Managers joined the channel in the bot (Mini App).
	Managers []int64
	// LastCheck is the latest member check, for the health indicator.
	LastCheck *processors.CheckSummary
	// JoinRequestMode handles requests to join (see JoinMode*).
	JoinRequestMode      string
	AccessChecker        CheckUserAccess     `json:",omitempty"`
	ScanReportProcessor  UserReportProcessor `json:",omitempty"`
	CleanReportProcessor UserReportProcessor `json:",omitempty"`
}

type ScanRequestType int

const (
	None ScanRequestType = iota
	AutoScan
	AutoClean
	Scan
	Clean
)

type ScanRequest struct {
	requestType     ScanRequestType
	channelInfo     ChannelInfo
	reportChannels  *[]int64
	accessChecker   CheckUserAccess
	reportProcessor UserReportProcessor
}
