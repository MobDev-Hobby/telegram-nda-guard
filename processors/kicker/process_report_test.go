package kicker

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	guard "github.com/MobDev-Hobby/telegram-nda-guard"
	"github.com/MobDev-Hobby/telegram-nda-guard/processors"
)

type call struct {
	op             string
	userID         int64
	revokeMessages bool
}

type fakeRestrictor struct {
	calls   []call
	failBan map[int64]bool
	reports []string
}

func (f *fakeRestrictor) Ban(_ context.Context, _, userID int64, revoke bool) error {
	f.calls = append(f.calls, call{"ban", userID, revoke})
	if f.failBan[userID] {
		return errors.New("USER_ADMIN_INVALID")
	}
	return nil
}

func (f *fakeRestrictor) Unban(_ context.Context, _, userID int64) error {
	f.calls = append(f.calls, call{op: "unban", userID: userID})
	return nil
}

func (f *fakeRestrictor) SendReport(_ context.Context, _ int64, text string) error {
	f.reports = append(f.reports, text)
	return nil
}

// With KeepBanned and CleanMessages both off the old code skipped the ban and
// only issued an OnlyIfBanned unban, a no-op, yet counted the user as kicked.
func TestKickAlwaysBansBeforeUnban(t *testing.T) {
	r := &fakeRestrictor{}
	k := New(r, WithCleanMessages(false), WithKeepBanned(false))

	results := k.KickUsers(context.Background(), guard.ChannelInfo{ID: -1001}, []guard.User{{ID: 5}}, nil)

	assert.Equal(t, []call{{"ban", 5, false}, {op: "unban", userID: 5}}, r.calls)
	assert.Equal(t, []processors.KickResult{{UserID: 5, OK: true}}, results)
}

func TestKickKeepBannedSkipsUnban(t *testing.T) {
	r := &fakeRestrictor{}
	k := New(r)

	k.KickUsers(context.Background(), guard.ChannelInfo{ID: -1001}, []guard.User{{ID: 5}},
		&processors.CleanOptions{KeepBanned: true, CleanMessages: true})

	assert.Equal(t, []call{{"ban", 5, true}}, r.calls)
}

func TestKickReportsPerUserFailure(t *testing.T) {
	r := &fakeRestrictor{failBan: map[int64]bool{6: true}}
	k := New(r)

	results := k.KickUsers(context.Background(), guard.ChannelInfo{ID: -1001}, []guard.User{{ID: 5}, {ID: 6}}, nil)

	assert.True(t, results[0].OK)
	assert.False(t, results[1].OK)
	assert.Contains(t, results[1].Error, "USER_ADMIN_INVALID")
}

func TestProcessReportUsesChannelOptions(t *testing.T) {
	r := &fakeRestrictor{}
	// Defaults would not touch unknown users; the channel override does.
	k := New(r, WithCleanUnknown(false))

	k.ProcessReport(context.Background(), processors.AccessReport{
		Channel:        guard.ChannelInfo{ID: -1001, Title: "News", Type: guard.ChatTypeChannel},
		ReportChannels: []int64{42},
		DeniedUsers:    []guard.User{{ID: 1}},
		UnknownUsers:   []guard.User{{ID: 2}},
		CleanOptions:   &processors.CleanOptions{CleanUnknown: true, KeepBanned: true},
		Stats:          guard.ScanStats{Fetched: 2, Total: 10},
	})

	assert.Equal(t, []call{{"ban", 1, false}, {"ban", 2, false}}, r.calls)
	assert.Len(t, r.reports, 1)
	assert.Contains(t, r.reports[0], "Clean report for channel News")
	assert.Contains(t, r.reports[0], "Kicked <b>2/2</b>")
	assert.Contains(t, r.reports[0], "only <b>2 of 10</b> members")
}
