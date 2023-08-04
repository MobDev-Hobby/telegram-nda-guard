package session_storage

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
			domain, err := New("/tmp", cryptor)
			assert.NoError(t, err)

			storage := domain.GetStorage("name")

			err = storage.StoreSession(ctx, data)
			assert.NoError(t, err)

			val, err := storage.LoadSession(ctx)
			assert.NoError(t, err)
			assert.Equal(t, val, data)
		},
	)

	t.Run(
		"domain init error", func(t *testing.T) {
			_, err := New("/tmp/../../", cryptor)
			assert.Error(t, err)
		},
	)

	t.Run(
		"gcm init error on save", func(t *testing.T) {

			ctrl := gomock.NewController(t)
			cryptorMock := NewMockCryproProvider(ctrl)
			cryptorMock.EXPECT().BlockSize().Return(1)

			domain, err := New("/tmp", cryptorMock)
			assert.NoError(t, err)

			storage := domain.GetStorage("name")

			err = storage.StoreSession(ctx, data)
			assert.Error(t, err)
		},
	)

	t.Run(
		"gcm init error on load", func(t *testing.T) {

			ctrl := gomock.NewController(t)
			cryptorMock := NewMockCryproProvider(ctrl)
			cryptorMock.EXPECT().BlockSize().Return(1)

			domain, err := New("/tmp", cryptorMock)
			assert.NoError(t, err)

			storage := domain.GetStorage("name")

			_, err = storage.LoadSession(ctx)
			assert.Error(t, err)
		},
	)

	t.Run(
		"file fallback", func(t *testing.T) {
			domain, err := New("/tmp", cryptor)
			assert.NoError(t, err)

			storage := domain.GetStorage("name")

			val, err := storage.LoadSession(ctx)
			assert.NoError(t, err)
			assert.Equal(t, val, data)
		},
	)

	t.Run(
		"empty session", func(t *testing.T) {
			domain, err := New("/tmp", cryptor)
			assert.NoError(t, err)

			storage := domain.GetStorage("name2")

			_, err = storage.LoadSession(ctx)
			assert.Error(t, err)
		},
	)

	t.Run(
		"wrong data error", func(t *testing.T) {
			domain, err := New("/tmp", cryptor)
			assert.NoError(t, err)

			storage := domain.GetStorage("name")

			err = storage.StoreSession(ctx, data)
			assert.NoError(t, err)

			storage.val = []byte("anything")
			_, err = storage.LoadSession(ctx)
			assert.Error(t, err)
		},
	)

	t.Run(
		"write file error", func(t *testing.T) {
			domain, err := New("/tmp", cryptor)
			assert.NoError(t, err)

			storage := domain.GetStorage("name")
			domain.dir = "/tmp/../../"

			err = storage.StoreSession(ctx, data)
			assert.Error(t, err)
		},
	)

	t.Run(
		"gen nonce error", func(t *testing.T) {
			domain, err := New("/tmp", cryptor)
			assert.NoError(t, err)

			domain.randomReader = strings.NewReader("1")
			storage := domain.GetStorage("name")

			err = storage.StoreSession(ctx, data)
			assert.Error(t, err)
		},
	)

	t.Run(
		"gcm open error", func(t *testing.T) {
			domain, err := New("/tmp", cryptor)
			assert.NoError(t, err)

			storage := domain.GetStorage("name")

			err = storage.StoreSession(ctx, data)
			assert.NoError(t, err)

			storage.val = make([]byte, 2048)
			_, err = io.ReadFull(rand.Reader, storage.val)
			assert.NoError(t, err)

			_, err = storage.LoadSession(ctx)
			assert.Error(t, err)
		},
	)
}
