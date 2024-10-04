package redis

import (
	"context"
	"crypto/cipher"
	"fmt"
	"io"
)

func (s *Domain) StoreSession(
	ctx context.Context,
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

	result := gcm.Seal(nonce, nonce, data, nil)

	err = s.redis.Set(ctx, s.getKeyName(name), result, 0)
	if err != nil {
		return fmt.Errorf("write redis error: %w", err)
	}

	return nil
}
