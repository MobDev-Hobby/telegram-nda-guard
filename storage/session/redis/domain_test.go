package redis

import (
	"context"
	"crypto/rand"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	ctx := context.Background()
	name := "name"
	data := []byte("my secret data")
	dataEncoded := []byte("encoded record")
	keyName := "tgSession:6e616d65"

	ctrl := gomock.NewController(t)
	redis := NewMockRedisClient(ctrl)
	cryptor := NewMockCryproProvider(ctrl)
	cryptor.EXPECT().Encrypt(data, gomock.Any()).AnyTimes()
	cryptor.EXPECT().Encrypt(make([]byte, 16), gomock.Any()).AnyTimes()
	cryptor.EXPECT().Decrypt(dataEncoded, gomock.Any()).AnyTimes()
	cryptor.EXPECT().BlockSize().Return(16).AnyTimes()

	buffers := make(map[string][]byte)
	redis.EXPECT().Set(ctx, keyName, gomock.Any(), time.Duration(0)).DoAndReturn(
		func(ctx context.Context, key string, data []byte, _ time.Duration) error {
			buffers[key] = data
			return nil
		},
	).AnyTimes()
	redis.EXPECT().Get(ctx, keyName).DoAndReturn(
		func(ctx context.Context, keyName string) ([]byte, error) {
			return buffers[keyName], nil
		},
	).AnyTimes()

	t.Run(
		"success", func(t *testing.T) {
			storage, err := New(cryptor, redis)
			assert.NoError(t, err)

			err = storage.StoreSession(ctx, name, data)
			assert.NoError(t, err)

			val, err := storage.LoadSession(ctx, name)
			assert.NoError(t, err)
			assert.Equal(t, val, data)
		},
	)

	t.Run(
		"gcm init error on save", func(t *testing.T) {
			ctrl := gomock.NewController(t)
			cryptorMock := NewMockCryproProvider(ctrl)
			cryptorMock.EXPECT().BlockSize().Return(1)

			storage, err := New(cryptorMock, nil)
			assert.NoError(t, err)

			err = storage.StoreSession(ctx, name, data)
			assert.Error(t, err)
		},
	)

	t.Run(
		"gcm init error on load", func(t *testing.T) {
			ctrl := gomock.NewController(t)
			cryptorMock := NewMockCryproProvider(ctrl)
			cryptorMock.EXPECT().BlockSize().Return(1)

			storage, err := New(cryptorMock, redis)
			assert.NoError(t, err)

			_, err = storage.LoadSession(ctx, name)
			assert.Error(t, err)
		},
	)

	t.Run(
		"empty session", func(t *testing.T) {
			redis.EXPECT().Get(ctx, gomock.Any()).Return(nil, nil)

			storage, err := New(cryptor, redis)
			assert.NoError(t, err)

			_, err = storage.LoadSession(ctx, "name2222")
			assert.Error(t, err)
		},
	)

	t.Run(
		"wrong data error", func(t *testing.T) {
			storage, err := New(cryptor, redis)
			assert.NoError(t, err)

			err = storage.StoreSession(ctx, name, data)
			assert.NoError(t, err)

			buffers[keyName] = []byte("wrong data")

			_, err = storage.LoadSession(ctx, name)
			assert.Error(t, err)
		},
	)

	t.Run(
		"redis write error", func(t *testing.T) {
			redis := NewMockRedisClient(ctrl)
			redis.EXPECT().Set(ctx, keyName, gomock.Any(), gomock.Any()).Return(errors.New("oops"))
			storage, err := New(cryptor, redis)
			assert.NoError(t, err)

			err = storage.StoreSession(ctx, name, data)
			assert.Error(t, err)
		},
	)

	t.Run(
		"gen nonce error", func(t *testing.T) {
			storage, err := New(cryptor, nil)
			assert.NoError(t, err)

			storage.randomReader = strings.NewReader("1")

			err = storage.StoreSession(ctx, name, data)
			assert.Error(t, err)
		},
	)

	t.Run(
		"gcm open error", func(t *testing.T) {
			storage, err := New(cryptor, redis)
			assert.NoError(t, err)

			err = storage.StoreSession(ctx, name, data)
			assert.NoError(t, err)

			buffers[keyName] = make([]byte, 2048)
			_, err = io.ReadFull(rand.Reader, buffers[keyName])
			assert.NoError(t, err)

			_, err = storage.LoadSession(ctx, name)
			assert.Error(t, err)
		},
	)
}
