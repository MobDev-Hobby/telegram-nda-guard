package bot

type UserInfo struct {
	FirstName string
	LastName  string
}

func (u UserInfo) GetFirstName() string {
	return u.FirstName
}

func (u UserInfo) GetLastName() string {
	return u.LastName
}
