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
	// Type is the Telegram chat type. It is one of the ChatType* constants
	// (e.g. ChatTypeChannel, ChatTypeSupergroup). An empty value means the type
	// has not been resolved yet; callers should treat that as "unknown".
	Type string
}

// Telegram chat type values, mirroring the "type" field returned by the Bot API
// getChat method. They let consumers distinguish broadcast channels from chats
// and groups (relevant for cleanup, which behaves the same but should be
// reported with the correct noun).
const (
	ChatTypePrivate    = "private"
	ChatTypeGroup      = "group"
	ChatTypeSupergroup = "supergroup"
	ChatTypeChannel    = "channel"
)

// ChatTypeNoun returns a human-readable noun for the given chat type suitable
// for embedding into user-facing messages (e.g. report headers). Unknown or
// empty values fall back to "chat".
func ChatTypeNoun(chatType string) string {
	if chatType == ChatTypeChannel {
		return "channel"
	}
	return "chat"
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
