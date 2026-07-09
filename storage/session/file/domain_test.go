package file

import (
	"context"
	"crypto/aes"
	"crypto/rand"
	"io"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	ctx := context.Background()
	data := []byte("my secret data")
	key := []byte("testtesttesttest")
	cryptor, err := aes.NewCipher(key)
	assert.NoError(t, err)

	// Use an isolated temp dir so the default ./data does not get created in
	// the package source tree and so concurrent subtests do not share state.
	tmpDir := t.TempDir()

	t.Run(
		"success", func(t *testing.T) {
			storage, err := New(cryptor, WithStoragePath(tmpDir))
			assert.NoError(t, err)

			err = storage.StoreSession(ctx, "name", data)
			assert.NoError(t, err)

			val, err := storage.LoadSession(ctx, "name")
			assert.NoError(t, err)
			assert.Equal(t, val, data)
		},
	)

	t.Run(
		"domain init error", func(t *testing.T) {
			_, err := New(cryptor, WithStoragePath("/tmp/../../"))
			assert.Error(t, err)
		},
	)

	t.Run(
		"gcm init error on save", func(t *testing.T) {
			ctrl := gomock.NewController(t)
			cryptorMock := NewMockCryproProvider(ctrl)
			cryptorMock.EXPECT().BlockSize().Return(1)

			storage, err := New(cryptorMock, WithStoragePath(tmpDir))
			assert.NoError(t, err)

			err = storage.StoreSession(ctx, "name", data)
			assert.Error(t, err)
		},
	)

	t.Run(
		"gcm init error on load", func(t *testing.T) {
			ctrl := gomock.NewController(t)
			cryptorMock := NewMockCryproProvider(ctrl)
			cryptorMock.EXPECT().BlockSize().Return(1)

			storage, err := New(cryptorMock, WithStoragePath(tmpDir))
			assert.NoError(t, err)

			_, err = storage.LoadSession(ctx, "name")
			assert.Error(t, err)
		},
	)

	t.Run(
		"file fallback", func(t *testing.T) {
			storage, err := New(cryptor, WithStoragePath(tmpDir))
			assert.NoError(t, err)

			val, err := storage.LoadSession(ctx, "name")
			assert.NoError(t, err)
			assert.Equal(t, val, data)
		},
	)

	t.Run(
		"empty session", func(t *testing.T) {
			storage, err := New(cryptor, WithStoragePath(tmpDir))
			assert.NoError(t, err)

			// A non-existent session returns an empty byte slice and no
			// error: the gotd session-storage contract treats an empty
			// payload as "no session yet" (first-run / fresh authorization).
			// Previously this asserted Error, but LoadSession deliberately
			// returns ([], nil) when the file is missing.
			val, err := storage.LoadSession(ctx, "name2222")
			assert.NoError(t, err)
			assert.Empty(t, val)
		},
	)

	t.Run(
		"wrong data error", func(t *testing.T) {
			storage, err := New(cryptor, WithStoragePath(tmpDir))
			assert.NoError(t, err)

			err = storage.StoreSession(ctx, "name", data)
			assert.NoError(t, err)

			storage.vals["name"] = []byte("anything")
			_, err = storage.LoadSession(ctx, "name")
			assert.Error(t, err)
		},
	)

	t.Run(
		"write file error", func(t *testing.T) {
			storage, err := New(cryptor, WithStoragePath(tmpDir))
			assert.NoError(t, err)

			storage.dir = "/tmp/../../"

			err = storage.StoreSession(ctx, "name", data)
			assert.Error(t, err)
		},
	)

	t.Run(
		"gen nonce error", func(t *testing.T) {
			storage, err := New(cryptor, WithStoragePath(tmpDir))
			assert.NoError(t, err)

			storage.randomReader = strings.NewReader("1")

			err = storage.StoreSession(ctx, "name", data)
			assert.Error(t, err)
		},
	)

	t.Run(
		"gcm open error", func(t *testing.T) {
			storage, err := New(cryptor, WithStoragePath(tmpDir))
			assert.NoError(t, err)

			err = storage.StoreSession(ctx, "name", data)
			assert.NoError(t, err)

			storage.vals["name"] = make([]byte, 2048)
			_, err = io.ReadFull(rand.Reader, storage.vals["name"])
			assert.NoError(t, err)

			_, err = storage.LoadSession(ctx, "name")
			assert.Error(t, err)
		},
	)
}
