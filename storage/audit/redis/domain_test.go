package redis

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MobDev-Hobby/telegram-nda-guard/storage/audit"
)

type memLists struct{ lists map[string][][]byte }

func (m *memLists) IsNil(error) bool { return false }
func (m *memLists) ListPush(_ context.Context, key string, value []byte, maxLen int64) error {
	l := append([][]byte{value}, m.lists[key]...)
	if int64(len(l)) > maxLen {
		l = l[:maxLen]
	}
	m.lists[key] = l
	return nil
}
func (m *memLists) ListRange(_ context.Context, key string, limit int64) ([][]byte, error) {
	l := m.lists[key]
	if int64(len(l)) > limit {
		l = l[:limit]
	}
	return l, nil
}

func TestAuditNewestFirstAndCapped(t *testing.T) {
	mem := &memLists{lists: map[string][][]byte{}}
	d := New(mem, WithMaxLen(3))
	ctx := context.Background()
	at := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	for i := range 5 {
		require.NoError(t, d.AppendAudit(ctx, -1001, audit.Event{At: at.Add(time.Duration(i) * time.Minute), Action: audit.ActionScanCompleted, Counts: map[string]int{"bad": i}}))
	}
	require.NoError(t, d.AppendAudit(ctx, -1002, audit.Event{Action: audit.ActionChannelJoined}))

	events, err := d.ListAudit(ctx, -1001, 10)
	require.NoError(t, err)
	require.Len(t, events, 3, "capped at maxLen")
	assert.Equal(t, 4, events[0].Counts["bad"], "newest first")
	assert.Contains(t, mem.lists, "pAudit:-1001")

	events, err = d.ListAudit(ctx, -1001, 2)
	require.NoError(t, err)
	assert.Len(t, events, 2)
}
