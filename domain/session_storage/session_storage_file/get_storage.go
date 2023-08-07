package session_storage_file

func (d *Domain) GetStorage(
	hash string,
) *Storage {
	return &Storage{
		sessionStorage: d,
		name:           hash,
	}
}
