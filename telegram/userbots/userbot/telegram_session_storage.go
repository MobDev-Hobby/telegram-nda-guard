package userbot

import "context"

type TelegramSessionStorage struct {
	name string
	d    *Domain
}

func (d *Domain) getSessionStorage(name string) *TelegramSessionStorage {
	return &TelegramSessionStorage{
		name: name,
		d:    d,
	}
}

func (s *TelegramSessionStorage) LoadSession(ctx context.Context) ([]byte, error) {
	return s.d.sessionStorage.LoadSession(ctx, s.name)
}

func (s *TelegramSessionStorage) StoreSession(ctx context.Context, data []byte) error {
	return s.d.sessionStorage.StoreSession(ctx, s.name, data)
}
