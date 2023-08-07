package report_processor

type AccessReport struct {
	ChannelId    int64
	AllowedUsers []User
	DeniedUsers  []User
	UnknownUsers []User
}

type User struct {
	ID        int64
	Firstname string
	LastName  string
	Username  string
	Phone     string
}
