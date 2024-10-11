package file

import (
	"context"
	"crypto/cipher"
	"fmt"
	"os"
)

func (s *Domain) LoadSession(
	_ context.Context,
	name string,
) ([]byte, error) {

	s.log.Debugf("try to load session: %s", name)
	if len(s.vals[name]) == 0 {
		s.log.Debugf("try to load session from file: %s", s.getFileName(name))
		var err error
		_, err = os.Stat(s.getFileName(name))
		if os.IsNotExist(err) {
			s.log.Debugf("file not exists: %s", s.getFileName(name))
			return []byte{}, nil
		}
		s.vals[name], err = os.ReadFile(s.getFileName(name))
		if err != nil {
			s.log.Errorf("file read error: %s, %s", name, err)
			return []byte{}, fmt.Errorf("read file error: %w", err)
		}
	}

	gcm, err := cipher.NewGCM(s.cryptor)
	if err != nil {
		s.log.Errorf("gcm init error: %s, %s", name, err)
		return nil, fmt.Errorf("gcm cipher error: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(s.vals[name]) < nonceSize {
		s.log.Errorf("stored data error: %s, %s", name, err)
		return nil, fmt.Errorf("stored text format error")
	}

	nonce, ciphertext := s.vals[name][:nonceSize], s.vals[name][nonceSize:]
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
