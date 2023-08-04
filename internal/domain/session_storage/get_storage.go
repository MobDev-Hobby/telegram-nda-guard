package session_storage

func (d *Domain) GetStorage(
	hash string,
) *Storage {
	return &Storage{
		sessionStorage: d,
		name:           hash,
	}
}
