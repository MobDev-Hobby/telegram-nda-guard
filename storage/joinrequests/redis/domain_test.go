package redis

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	knownredis "github.com/MobDev-Hobby/telegram-nda-guard/storage/knownchats/redis"

	"github.com/MobDev-Hobby/telegram-nda-guard/storage/joinrequests"
	"github.com/MobDev-Hobby/telegram-nda-guard/storage/knownchats"
)

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

func TestJoinRequestsPerChat(t *testing.T) {
	mem := &memRedis{hashes: map[string]map[string][]byte{}}
	d := New(mem)
	ctx := context.Background()
	at := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	r := joinrequests.Request{UserID: 7, FirstName: "Ann", RequestedAt: at, Check: "bad", CheckedAt: at}

	require.NoError(t, d.StoreJoinRequest(ctx, -1001, r))
	got, err := d.LoadJoinRequests(ctx, -1001)
	require.NoError(t, err)
	assert.Equal(t, []joinrequests.Request{r}, got)
	assert.Contains(t, mem.hashes, "pJoinRequests:-1001")

	require.NoError(t, d.DropJoinRequest(ctx, -1001, 7))
	got, err = d.LoadJoinRequests(ctx, -1001)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestKnownChatsRoundTrip(t *testing.T) {
	mem := &memRedis{hashes: map[string]map[string][]byte{}}
	d := knownredis.New(mem)
	ctx := context.Background()
	c := knownchats.Chat{ID: -1002, Title: "News", Type: "channel", AddedBy: 5, AddedAt: time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC), CanRestrictMembers: true}

	require.NoError(t, d.StoreKnownChat(ctx, c))
	got, err := d.LoadKnownChats(ctx)
	require.NoError(t, err)
	assert.Equal(t, []knownchats.Chat{c}, got)
	require.NoError(t, d.DropKnownChat(ctx, c.ID))
	got, err = d.LoadKnownChats(ctx)
	require.NoError(t, err)
	assert.Empty(t, got)
}
