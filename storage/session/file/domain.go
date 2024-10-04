package file

import (
	"crypto/cipher"
	"crypto/rand"
	"io"

	"go.uber.org/zap"
	"golang.org/x/sys/unix"
)

type Domain struct {
	log          Logger
	cryptor      cipher.Block
	dir          string
	randomReader io.Reader
	vals         map[string][]byte
}

func New(
	cryptor CryproProvider,
	opts ...SSOpts,
) (*Domain, error) {

	d := &Domain{
		log:          Logger(zap.NewNop().Sugar()),
		dir:          "/tmp",
		cryptor:      cryptor,
		randomReader: rand.Reader,
		vals:         make(map[string][]byte),
	}

	for _, opt := range opts {
		opt(d)
	}

	if err := unix.Access(d.dir, unix.W_OK); err != nil {
		return nil, err
	}
	return d, nil
}
