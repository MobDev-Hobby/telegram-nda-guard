package file

import (
	"crypto/cipher"
	"crypto/rand"
	"io"
	"os"

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
		log: Logger(zap.NewNop().Sugar()),
		// Default moved out of world-traversable /tmp: the session blob is the
		// auth key used to impersonate the bot, so it must live in an
		// owner-only directory. Callers can override via WithStoragePath.
		dir:          "./data",
		cryptor:      cryptor,
		randomReader: rand.Reader,
		vals:         make(map[string][]byte),
	}

	for _, opt := range opts {
		opt(d)
	}

	// Ensure the directory exists and is owner-only (0700). MkdirAll honors a
	// umask, so explicitly chmod afterwards to guarantee the mode.
	if err := os.MkdirAll(d.dir, 0o700); err != nil {
		return nil, err
	}
	if err := os.Chmod(d.dir, 0o700); err != nil {
		return nil, err
	}
	if err := unix.Access(d.dir, unix.W_OK); err != nil {
		return nil, err
	}
	return d, nil
}
