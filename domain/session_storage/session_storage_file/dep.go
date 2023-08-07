package session_storage_file

import (
	"crypto/cipher"

	"github.com/MobDev-Hobby/telegram-nda-guard/interfaces"
)

//go:generate mockgen -source dep.go -destination ./dep_mock_test.go -package ${GOPACKAGE}

type Logger interfaces.Logger

type CryproProvider interface {
	cipher.Block
}
