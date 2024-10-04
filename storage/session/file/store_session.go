package file

import (
	"context"
	"crypto/cipher"
	"fmt"
	"io"
	"os"
)

func (s *Domain) StoreSession(
	_ context.Context,
	name string,
	data []byte,
) error {

	gcm, err := cipher.NewGCM(s.cryptor)
	if err != nil {
		return fmt.Errorf("gcm init error: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(s.randomReader, nonce); err != nil {
		return fmt.Errorf("nonce gen error: %w", err)
	}

	s.vals[name] = gcm.Seal(nonce, nonce, data, nil)

	err = os.WriteFile(s.getFileName(name), s.vals[name], 0600)
	if err != nil {
		return fmt.Errorf("write file error: %w", err)
	}

	return nil
}
