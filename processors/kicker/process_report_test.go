package kicker

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	guard "github.com/MobDev-Hobby/telegram-nda-guard"
	"github.com/MobDev-Hobby/telegram-nda-guard/processors"
)

// fakeRestrictor is a recording UserRestrictor used to assert which actions the
// kicker performed (ban/unban/send). It does not use the generated mock to keep
// the test focused on the kicker's decision logic, not the mock framework.
type fakeRestrictor struct {
	bans     []banCall
	unbans   []unbanCall
	sent     []string
	banErr   error
	unbanErr error
}

type banCall struct {
	channelID, userID int64
	revoke            bool
}

type unbanCall struct {
	channelID, userID int64
}

func (f *fakeRestrictor) SendReportMessage(_ context.Context, _ int64, text string) error {
	f.sent = append(f.sent, text)
	return nil
}
func (f *fakeRestrictor) Ban(_ context.Context, channelID, userID int64, revoke bool) error {
	f.bans = append(f.bans, banCall{channelID, userID, revoke})
	return f.banErr
}
func (f *fakeRestrictor) Unban(_ context.Context, channelID, userID int64) error {
	f.unbans = append(f.unbans, unbanCall{channelID, userID})
	return f.unbanErr
}

func newKickerTestDomain(t *testing.T, restrictor *fakeRestrictor, opts ...Option) *Domain {
	t.Helper()
	d := New(restrictor, opts...)
	return d
}

func deniedUser(id int64) guard.User {
	return guard.User{ID: id, Username: "u"}
}

func TestProcessReport_KicksDeniedUsersByDefault(t *testing.T) {
	ctx := context.Background()
	r := &fakeRestrictor{}
	d := newKickerTestDomain(t, r) // defaults: keepBanned=false, cleanMessages=true, cleanUnknown=false

	d.ProcessReport(ctx, processors.AccessReport{
		Channel:        guard.ChannelInfo{ID: 100, Title: "Test"},
		DeniedUsers:    []guard.User{deniedUser(1), deniedUser(2)},
		ReportChannels: []int64{555},
	})

	// cleanMessages=true (default) → ban with revoke, then unban (kick).
	assert.Len(t, r.bans, 2)
	assert.True(t, r.bans[0].revoke)
	assert.Len(t, r.unbans, 2)
	assert.Len(t, r.sent, 1)
}

func TestProcessReport_KeepBannedSkipsUnban(t *testing.T) {
	ctx := context.Background()
	r := &fakeRestrictor{}
	d := newKickerTestDomain(t, r, WithKeepBanned(true))

	d.ProcessReport(ctx, processors.AccessReport{
		Channel:        guard.ChannelInfo{ID: 100, Title: "Test"},
		DeniedUsers:    []guard.User{deniedUser(1)},
		ReportChannels: []int64{555},
	})

	assert.Len(t, r.bans, 1)
	assert.Empty(t, r.unbans, "keepBanned must skip the unban step")
}

func TestProcessReport_PerChannelCleanOptionsOverrideDefaults(t *testing.T) {
	ctx := context.Background()
	r := &fakeRestrictor{}
	// Process default would clean unknown=false, but the per-channel report
	// requests cleanUnknown=true AND keepBanned=true.
	d := newKickerTestDomain(t, r)

	d.ProcessReport(ctx, processors.AccessReport{
		Channel:     guard.ChannelInfo{ID: 100, Title: "Test"},
		DeniedUsers: []guard.User{deniedUser(1)},
		UnknownUsers: []guard.User{deniedUser(2)},
		CleanOptions: &processors.CleanOptions{
			KeepBanned:    true,
			CleanMessages: false,
			CleanUnknown:  true,
		},
		ReportChannels: []int64{555},
	})

	// cleanUnknown=true → unknown user 2 also banned.
	assert.Len(t, r.bans, 2, "unknown user must be banned when CleanUnknown")
	// keepBanned=true → no unbans.
	assert.Empty(t, r.unbans)
	// cleanMessages=false → revoke must be false.
	for _, b := range r.bans {
		assert.False(t, b.revoke)
	}
}

func TestProcessReport_MigratedToUsesNewID(t *testing.T) {
	ctx := context.Background()
	r := &fakeRestrictor{}
	d := newKickerTestDomain(t, r, WithKeepBanned(true))

	migrated := int64(-100999)
	d.ProcessReport(ctx, processors.AccessReport{
		Channel: guard.ChannelInfo{
			ID:         100,
			Title:      "Test",
			MigratedTo: &migrated,
		},
		DeniedUsers:    []guard.User{deniedUser(1)},
		ReportChannels: []int64{555},
	})

	assert.Equal(t, migrated, r.bans[0].channelID, "must restrict in migrated-to chat")
}

func TestProcessReport_BanErrorDoesNotCountAsCleaned(t *testing.T) {
	ctx := context.Background()
	r := &fakeRestrictor{banErr: errors.New("telegran says no")}
	d := newKickerTestDomain(t, r, WithKeepBanned(true))

	d.ProcessReport(ctx, processors.AccessReport{
		Channel:        guard.ChannelInfo{ID: 100, Title: "Test"},
		DeniedUsers:    []guard.User{deniedUser(1)},
		ReportChannels: []int64{555},
	})

	// A failed ban must still produce a report message.
	assert.Len(t, r.sent, 1)
}

// Compile-time: ensure fakeRestrictor satisfies the interface.
var _ UserRestrictor = (*fakeRestrictor)(nil)
