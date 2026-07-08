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
	ID                   int64
	CommandChannelIDs    []int64
	AutoScan             bool
	AutoClean            bool
	AllowClean           bool
	AccessChecker        CheckUserAccess     `json:",omitempty"`
	ScanReportProcessor  UserReportProcessor `json:",omitempty"`
	CleanReportProcessor UserReportProcessor `json:",omitempty"`
	// CleanOptions optionally overrides the cleaner processor's process-wide
	// defaults for this channel (e.g. keep-banned, clean-messages). When nil,
	// the processor's own defaults apply. Ignored by scan-only processors.
	CleanOptions *processors.CleanOptions `json:",omitempty"`
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
	// cleanOptions is forwarded to the cleaner processor via AccessReport so
	// per-channel cleanup behavior overrides the processor's defaults.
	cleanOptions *processors.CleanOptions
}
