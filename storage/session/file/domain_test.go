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

	t.Run(
		"success", func(t *testing.T) {
			storage, err := New(cryptor)
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

			storage, err := New(cryptorMock)
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

			storage, err := New(cryptorMock)
			assert.NoError(t, err)

			_, err = storage.LoadSession(ctx, "name")
			assert.Error(t, err)
		},
	)

	t.Run(
		"file fallback", func(t *testing.T) {
			storage, err := New(cryptor)
			assert.NoError(t, err)

			val, err := storage.LoadSession(ctx, "name")
			assert.NoError(t, err)
			assert.Equal(t, val, data)
		},
	)

	t.Run(
		"empty session", func(t *testing.T) {
			storage, err := New(cryptor)
			assert.NoError(t, err)

			_, err = storage.LoadSession(ctx, "name2222")
			assert.Error(t, err)
		},
	)

	t.Run(
		"wrong data error", func(t *testing.T) {
			storage, err := New(cryptor)
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
			storage, err := New(cryptor)
			assert.NoError(t, err)

			storage.dir = "/tmp/../../"

			err = storage.StoreSession(ctx, "name", data)
			assert.Error(t, err)
		},
	)

	t.Run(
		"gen nonce error", func(t *testing.T) {
			storage, err := New(cryptor)
			assert.NoError(t, err)

			storage.randomReader = strings.NewReader("1")

			err = storage.StoreSession(ctx, "name", data)
			assert.Error(t, err)
		},
	)

	t.Run(
		"gcm open error", func(t *testing.T) {
			storage, err := New(cryptor)
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
