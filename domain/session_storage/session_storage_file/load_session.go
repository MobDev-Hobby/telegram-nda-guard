package session_storage_file

import (
	"context"
	"crypto/cipher"
	"fmt"
	"os"
)

func (s *Storage) LoadSession(
	_ context.Context,
) ([]byte, error) {
	s.sessionStorage.log.Debugf("try to load session: %s", s.name)
	if len(s.val) == 0 {
		s.sessionStorage.log.Debugf("try to load session from file: %s", s.name)
		var err error
		_, err = os.Stat(s.getFileName())
		if os.IsNotExist(err) {
			s.sessionStorage.log.Debugf("file not exists: %s", s.name)
			return nil, nil
		}
		s.val, err = os.ReadFile(s.getFileName())
		if err != nil {
			s.sessionStorage.log.Errorf("file read error: %s, %s", s.name, err)
			return []byte{}, fmt.Errorf("read file error: %w", err)
		}
	}

	gcm, err := cipher.NewGCM(s.sessionStorage.cryptor)
	if err != nil {
		s.sessionStorage.log.Errorf("gcm init error: %s, %s", s.name, err)
		return nil, fmt.Errorf("gcm cipher error: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(s.val) < nonceSize {
		s.sessionStorage.log.Errorf("stored data error: %s, %s", s.name, err)
		return nil, fmt.Errorf("stored text format error")
	}

	nonce, ciphertext := s.val[:nonceSize], s.val[nonceSize:]
	plaintext, err := gcm.Open(
		nil,
		nonce,
		ciphertext,
		nil,
	)
	if err != nil {
		s.sessionStorage.log.Errorf("gcm open error: %s, %s", s.name, err)
		return nil, fmt.Errorf("gcm open error: %w", err)
	}

	s.sessionStorage.log.Debugf("session loaded: %s", s.name)
	return plaintext, nil
}
