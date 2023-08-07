package session_storage_file

import (
	"context"
	"crypto/cipher"
	"fmt"
	"io"
	"os"
)

func (s *Storage) StoreSession(
	_ context.Context,
	data []byte,
) error {
	gcm, err := cipher.NewGCM(s.sessionStorage.cryptor)
	if err != nil {
		return fmt.Errorf("gcm init error: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(s.sessionStorage.randomReader, nonce); err != nil {
		return fmt.Errorf("nonce gen error: %w", err)
	}

	s.val = gcm.Seal(nonce, nonce, data, nil)

	err = os.WriteFile(s.getFileName(), s.val, 0644)
	if err != nil {
		return fmt.Errorf("write file error: %w", err)
	}

	return nil
}
