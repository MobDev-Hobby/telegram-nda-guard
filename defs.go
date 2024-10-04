package guard

type User struct {
	ID        int64
	FirstName string
	LastName  string
	Username  string
	Phone     string
}

type Permission int32

const (
	CanInviteUsers Permission = iota
	CanPromoteMembers
	CanRestrictMembers
)

type ChannelInfo struct {
	ID         int64
	Title      string
	MigratedTo *int64
}

type Message struct {
	ChatType string
	ChatID   int64
	ThreadID *int
	Text     string
}

type Update struct {
	Message *Message
}
