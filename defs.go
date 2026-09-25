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
	// URL, when set, turns the button into a link button (Command is ignored).
	URL string
	// WebAppURL, when set, opens a Telegram Mini App. Telegram only allows
	// web_app inline buttons in private chats; in groups use URL with a
	// t.me/<bot>/<app> direct link instead.
	WebAppURL string
}

type Button struct {
	ID   int32
	Text string
	// RequestChannel turns the button into a request_chat button: pressing it
	// lets the user pick a chat and share it with the bot.
	RequestChannel *bool
	// RequestChatIsChannel selects which chats the request_chat picker shows:
	// true lists broadcast channels, false lists groups and supergroups.
	// Telegram never shows both kinds in one picker.
	RequestChatIsChannel bool
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
	ID   string
	Data string
	// From is the user who pressed the button. Message.User is the author of
	// the message carrying the button, which is the bot itself, so
	// authorization must use From.
	From    User
	Message *MessageReceived
}

type CallbackResponse struct {
	ID        string
	Text      string
	ShowAlert bool
}

// ScanStats describes how complete a member listing is. Telegram does not
// always return every member of a broadcast channel, so Fetched can be lower
// than Total; consumers should surface that instead of treating the list as
// complete.
type ScanStats struct {
	Fetched int `json:"fetched"`
	Total   int `json:"total"`
}

// Partial reports whether fewer members were fetched than Telegram reports.
func (s ScanStats) Partial() bool {
	return s.Total > s.Fetched
}
