package redis

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/MobDev-Hobby/telegram-nda-guard/storage/channels"
)

// newTestStorage builds a Domain backed by an in-memory hash that emulates the
// RedisClient contract for Store/LoadAll/Drop. It returns the storage and the
// underlying buffer so individual cases can assert on or seed state.
func newTestStorage(t *testing.T) (*Domain, map[string][]byte) {
	t.Helper()
	ctrl := gomock.NewController(t)

	buffers := make(map[string][]byte)
	redis := NewMockRedisClient(ctrl)

	redis.EXPECT().IsNil(gomock.Any()).Return(false).AnyTimes()

	redis.EXPECT().SetHashValue(
		gomock.Any(), "pChannel", gomock.Any(), gomock.Any(), gomock.Any(),
	).DoAndReturn(
		func(_ context.Context, _, key string, value []byte, _ time.Duration) error {
			buffers[key] = value
			return nil
		},
	).AnyTimes()

	redis.EXPECT().GetAllHashValues(gomock.Any(), "pChannel").DoAndReturn(
		func(_ context.Context, _ string) (map[string][]byte, error) {
			cp := make(map[string][]byte, len(buffers))
			for k, v := range buffers {
				cp[k] = v
			}
			return cp, nil
		},
	).AnyTimes()

	redis.EXPECT().DropHashValue(gomock.Any(), "pChannel", gomock.Any()).DoAndReturn(
		func(_ context.Context, _, key string) error {
			delete(buffers, key)
			return nil
		},
	).AnyTimes()

	storage, err := New(redis)
	assert.NoError(t, err)
	return storage, buffers
}

func TestStoreLoadAll(t *testing.T) {
	ctx := context.Background()
	storage, _ := newTestStorage(t)

	err := storage.Store(ctx, makeProtectedChannel(123, []int64{555}, true, false, true))
	assert.NoError(t, err)

	err = storage.Store(ctx, makeProtectedChannel(456, []int64{555}, false, true, false))
	assert.NoError(t, err)

	loaded, err := storage.LoadAll(ctx)
	assert.NoError(t, err)
	assert.Len(t, loaded, 2)
}

func TestDropRemovesRecord(t *testing.T) {
	ctx := context.Background()
	storage, _ := newTestStorage(t)

	assert.NoError(t, storage.Store(ctx, makeProtectedChannel(123, []int64{555}, true, false, true)))

	loaded, err := storage.LoadAll(ctx)
	assert.NoError(t, err)
	assert.Len(t, loaded, 1)

	// Drop must remove the persisted record.
	err = storage.Drop(ctx, 123)
	assert.NoError(t, err)

	loaded, err = storage.LoadAll(ctx)
	assert.NoError(t, err)
	assert.Empty(t, loaded)
}

func TestDropIsIdempotent(t *testing.T) {
	ctx := context.Background()
	storage, _ := newTestStorage(t)

	// Dropping a channel that was never stored must not error.
	err := storage.Drop(ctx, 999)
	assert.NoError(t, err)
}

func TestDropRedisErrorPropagates(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	redis := NewMockRedisClient(ctrl)

	redis.EXPECT().DropHashValue(
		gomock.Any(), "pChannel", strconv.FormatInt(42, 10),
	).Return(errors.New("redis down"))

	storage, err := New(redis)
	assert.NoError(t, err)

	err = storage.Drop(ctx, 42)
	assert.Error(t, err)
}

// makeProtectedChannel is a small helper kept in the redis test package to avoid
// repeating the struct literal across cases.
func makeProtectedChannel(id int64, commandChannels []int64, autoScan, autoClean, allowClean bool) *channels.ProtectedChannel {
	return &channels.ProtectedChannel{
		ID:                id,
		CommandChannelIDs: commandChannels,
		AutoScan:          autoScan,
		AutoClean:         autoClean,
		AllowClean:        allowClean,
	}
}
