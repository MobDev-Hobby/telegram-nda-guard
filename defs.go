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

type InlineButton struct {
	Text    string
	Command string
}

type Button struct {
	ID             int32
	Text           string
	RequestChannel *bool
}

type Message struct {
	ChatType      string
	ChatID        int64
	ThreadID      *int
	Text          string
	InlineButtons [][]InlineButton
	Buttons       [][]Button
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
	Message       *MessageReceived
	CallbackQuery *CallbackQuery
}

type CallbackQuery struct {
	ID      string
	Data    string
	Message *MessageReceived
}

type CallbackResponse struct {
	ID        string
	Text      string
	ShowAlert bool
}
