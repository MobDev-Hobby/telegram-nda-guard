package session_storage_file

import (
	"crypto/cipher"
	"crypto/rand"
	"io"

	"go.uber.org/zap"
	"golang.org/x/sys/unix"
)

type Domain struct {
	log     Logger
	cryptor cipher.Block
	dir          string
	randomReader io.Reader
}

func New(
	dir string,
	cryptor CryproProvider,
	log Logger,
) (*Domain, error) {
	if err := unix.Access(dir, unix.W_OK); err != nil {
		return nil, err
	}
	logger := Logger(zap.NewNop().Sugar())
	if log != nil {
		logger = log
	}
	return &Domain{
		log:          logger,
		dir:          dir,
		cryptor:      cryptor,
		randomReader: rand.Reader,
	}, nil
}
