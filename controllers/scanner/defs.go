package scanner

type ChannelInfo struct {
	migratedFrom      *int64
	id                int64
	commandChannelIDs []int64
	title             string
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
	AccessChecker        CheckUserAccess
	ScanReportProcessor  UserReportProcessor
	CleanReportProcessor UserReportProcessor
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
