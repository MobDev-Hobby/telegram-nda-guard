package session_storage

import (
	"crypto/cipher"
)

//go:generate mockgen -source dep.go -destination ./dep_mock_test.go -package ${GOPACKAGE}

type CryproProvider interface {
	cipher.Block
}
