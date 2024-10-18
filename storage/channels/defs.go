package channels

type ProtectedChannel struct {
	ID                int64
	CommandChannelIDs []int64
	AutoScan          bool
	AutoClean         bool
	AllowClean        bool
}
