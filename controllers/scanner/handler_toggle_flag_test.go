package scanner

import (
	"context"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/MobDev-Hobby/telegram-nda-guard/storage/channels"
)

// memoryStorage is a minimal in-memory ProtectedChannelStorage for testing the
// toggle logic without redis. It records the last stored channel per id and any
// dropped ids.
type memoryStorage struct {
	mu       sync.Mutex
	stored   map[int64]channels.ProtectedChannel
	dropped  []int64
	storeErr error
}

func newMemoryStorage() *memoryStorage {
	return &memoryStorage{stored: map[int64]channels.ProtectedChannel{}}
}

func (m *memoryStorage) LoadAll(_ context.Context) ([]channels.ProtectedChannel, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]channels.ProtectedChannel, 0, len(m.stored))
	for _, c := range m.stored {
		out = append(out, c)
	}
	return out, nil
}

func (m *memoryStorage) Store(_ context.Context, c *channels.ProtectedChannel) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.storeErr != nil {
		return m.storeErr
	}
	m.stored[c.ID] = *c
	return nil
}

func (m *memoryStorage) Drop(_ context.Context, id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dropped = append(m.dropped, id)
	delete(m.stored, id)
	return nil
}

func (m *memoryStorage) get(id int64) (channels.ProtectedChannel, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.stored[id]
	return c, ok
}

// newTestDomain builds a scanner Domain wired only with enough state to exercise
// applyFlagToggle: a protected channel and an in-memory storage. No telegram or
// user bot is required because applyFlagToggle never touches them.
func newTestDomain(t *testing.T, storage *memoryStorage) *Domain {
	t.Helper()
	d := &Domain{
		storage:                 storage,
		log:                     zap.NewNop().Sugar(),
		protectedChannels:       map[int64]ProtectedChannel{},
		channels:                map[int64]ChannelInfo{},
		commandChannels:         map[int64][]int64{},
		channelAutoScanInterval: time.Hour, // valid duration; ticker path covered separately
	}
	return d
}

func TestApplyFlagToggle_FlipsAndPersists(t *testing.T) {
	storage := newMemoryStorage()
	d := newTestDomain(t, storage)
	d.protectedChannels[100] = ProtectedChannel{
		ID:                100,
		CommandChannelIDs: []int64{5},
		AutoScan:          true,
		AutoClean:         false,
		AllowClean:        true,
	}

	err := d.applyFlagToggle(context.Background(), 100, flagAutoClean)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := d.protectedChannels[100]
	if !got.AutoClean {
		t.Errorf("AutoClean should be true after toggle, got false")
	}
	if !got.AutoScan {
		t.Errorf("AutoScan should remain true, got false")
	}

	persisted, ok := storage.get(100)
	if !ok {
		t.Fatalf("channel 100 not persisted")
	}
	if !persisted.AutoClean {
		t.Errorf("AutoClean not persisted as true")
	}
}

func TestApplyFlagToggle_TogglesBack(t *testing.T) {
	storage := newMemoryStorage()
	d := newTestDomain(t, storage)
	d.protectedChannels[100] = ProtectedChannel{ID: 100, AllowClean: true}

	_ = d.applyFlagToggle(context.Background(), 100, flagAllowClean)
	if d.protectedChannels[100].AllowClean {
		t.Errorf("AllowClean should be false after one toggle")
	}
	_ = d.applyFlagToggle(context.Background(), 100, flagAllowClean)
	if !d.protectedChannels[100].AllowClean {
		t.Errorf("AllowClean should be true after two toggles")
	}
}

func TestApplyFlagToggle_UnknownFlagRejected(t *testing.T) {
	storage := newMemoryStorage()
	d := newTestDomain(t, storage)
	d.protectedChannels[100] = ProtectedChannel{ID: 100}

	err := d.applyFlagToggle(context.Background(), 100, "bogus")
	if err == nil {
		t.Errorf("expected error for unknown flag")
	}
}

func TestApplyFlagToggle_UnknownChannelRejected(t *testing.T) {
	storage := newMemoryStorage()
	d := newTestDomain(t, storage)

	err := d.applyFlagToggle(context.Background(), 999, flagAutoScan)
	if err == nil {
		t.Errorf("expected error for unknown channel")
	}
}

func TestParseSetFlagPayload(t *testing.T) {
	cases := []struct {
		in       string
		wantID   int64
		wantFlag string
		wantOK   bool
	}{
		{"/setflag 100 autoscan", 100, flagAutoScan, true},
		{"/setflag -100123 autoclean", -100123, flagAutoClean, true},
		{"/settings 100", 0, "", false},
		{"/setflag abc autoscan", 0, "", false},
		{"", 0, "", false},
	}
	for _, tc := range cases {
		id, flag, ok := parseSetFlagPayload(tc.in)
		if ok != tc.wantOK || id != tc.wantID || flag != tc.wantFlag {
			t.Errorf("parseSetFlagPayload(%q) = (%d, %q, %v), want (%d, %q, %v)",
				tc.in, id, flag, ok, tc.wantID, tc.wantFlag, tc.wantOK)
		}
	}
}

// Compile-time: memoryStorage satisfies the (GLM-2) extended storage interface.
var _ ProtectedChannelStorageWithDrop = (*memoryStorage)(nil)

// ProtectedChannelStorageWithDrop is the storage contract including Drop, as it
// will exist once GLM-2 lands. Declared here only so the test's memoryStorage
// is checked against the future interface; on this branch (pre-GLM-2) it is a
// superset and still satisfied.
type ProtectedChannelStorageWithDrop interface {
	LoadAll(context.Context) ([]channels.ProtectedChannel, error)
	Store(context.Context, *channels.ProtectedChannel) error
	Drop(context.Context, int64) error
}
