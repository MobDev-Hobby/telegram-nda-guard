package guard

type User struct {
	ID        int64
	FirstName string
	LastName  string
	Username  string
	Usernames []string
	Phone     *string
}

type Permission int32

const (
	CanInviteUsers Permission = iota
	CanPromoteMembers
	CanRestrictMembers
)

type ChannelInfo struct {
	ID         int64
	MigratedTo *int64
	Title      string
}

type Message struct {
	ChatType string
	ChatID   int64
	ThreadID *int
	Text     string
}

type ChatShared struct {
	ChatID    int64
	RequestID int
}

type MessageReceived struct {
	Message
	ChatShared *ChatShared
	User       User
}

type Update struct {
	Message *MessageReceived
}
