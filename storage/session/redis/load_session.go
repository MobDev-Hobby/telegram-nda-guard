package redis

import (
	"context"
	"crypto/cipher"
	"fmt"
)

func (s *Domain) LoadSession(
	ctx context.Context,
	name string,
) ([]byte, error) {

	s.log.Debugf("try to load session from redis: %s", s.getKeyName(name))

	result, err := s.redis.Get(ctx, s.getKeyName(name))
	if err != nil {
		s.log.Errorf("redis read error: %s, %s", name, err)
		return []byte{}, fmt.Errorf("read redis error: %w", err)
	}

	gcm, err := cipher.NewGCM(s.cryptor)
	if err != nil {
		s.log.Errorf("gcm init error: %s, %s", name, err)
		return nil, fmt.Errorf("gcm cipher error: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(result) < nonceSize {
		s.log.Errorf("stored data error: %s, %s", name, err)
		return nil, fmt.Errorf("stored text format error")
	}

	nonce, ciphertext := result[:nonceSize], result[nonceSize:]
	// #nosec G407
	plaintext, err := gcm.Open(
		nil,
		nonce,
		ciphertext,
		nil,
	)
	if err != nil {
		s.log.Errorf("gcm open error: %s, %s", name, err)
		return nil, fmt.Errorf("gcm open error: %w", err)
	}

	s.log.Debugf("session loaded: %s", name)
	return plaintext, nil
}
