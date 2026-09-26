package redis

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MobDev-Hobby/telegram-nda-guard/storage/whitelist"
)

// memRedis emulates Redis hashes: hash -> key -> value.
type memRedis struct{ hashes map[string]map[string][]byte }

func (m *memRedis) IsNil(error) bool { return false }
func (m *memRedis) DropHashValue(_ context.Context, hash, key string) error {
	delete(m.hashes[hash], key)
	return nil
}
func (m *memRedis) SetHashValue(_ context.Context, hash, key string, value []byte, _ time.Duration) error {
	if m.hashes[hash] == nil {
		m.hashes[hash] = map[string][]byte{}
	}
	m.hashes[hash][key] = value
	return nil
}
func (m *memRedis) GetAllHashValues(_ context.Context, hash string) (map[string][]byte, error) {
	return m.hashes[hash], nil
}

func TestWhitelistRoundTripPerChannel(t *testing.T) {
	mem := &memRedis{hashes: map[string]map[string][]byte{}}
	d := New(mem)
	ctx := context.Background()
	at := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	entry := whitelist.Entry{UserID: 7, FirstName: "Ann", ApprovedBy: 1, ApprovedAt: at, ExpiresAt: at.Add(30 * 24 * time.Hour)}

	require.NoError(t, d.StoreWhitelistEntry(ctx, -1001, entry))
	require.NoError(t, d.StoreWhitelistEntry(ctx, -1002, whitelist.Entry{UserID: 8}))

	got, err := d.LoadWhitelist(ctx, -1001)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, entry, got[0])
	assert.Contains(t, mem.hashes, "pWhitelist:-1001")

	require.NoError(t, d.DropWhitelistEntry(ctx, -1001, 7))
	got, err = d.LoadWhitelist(ctx, -1001)
	require.NoError(t, err)
	assert.Empty(t, got)
}
