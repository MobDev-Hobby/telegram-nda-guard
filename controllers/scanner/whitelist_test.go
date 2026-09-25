package scanner

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	guard "github.com/MobDev-Hobby/telegram-nda-guard"
	"github.com/MobDev-Hobby/telegram-nda-guard/storage/whitelist"
)

type memWhitelist struct {
	entries map[int64]map[int64]whitelist.Entry
	loads   int
}

func (m *memWhitelist) LoadWhitelist(_ context.Context, channelID int64) ([]whitelist.Entry, error) {
	m.loads++
	out := []whitelist.Entry{}
	for _, e := range m.entries[channelID] {
		out = append(out, e)
	}
	return out, nil
}
func (m *memWhitelist) StoreWhitelistEntry(_ context.Context, channelID int64, e whitelist.Entry) error {
	if m.entries[channelID] == nil {
		m.entries[channelID] = map[int64]whitelist.Entry{}
	}
	m.entries[channelID][e.UserID] = e
	return nil
}
func (m *memWhitelist) DropWhitelistEntry(_ context.Context, channelID, userID int64) error {
	delete(m.entries[channelID], userID)
	return nil
}

type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func newWhitelistDomain(t *testing.T) (*Domain, *fakeBot, *memWhitelist, *clock) {
	t.Helper()
	d, bot, _, _, _ := newMiniAppDomain(t, true)
	wl := &memWhitelist{entries: map[int64]map[int64]whitelist.Entry{}}
	d.whitelistStorage = wl
	c := &clock{t: time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)}
	d.now = c.now
	return d, bot, wl, c
}

func scanUser(t *testing.T, d *Domain, userID int64) (ScanView, ScannedUser) {
	t.Helper()
	started, err := d.StartChannelScan(context.Background(), testChannelID, testAdminID)
	require.NoError(t, err)
	view := waitScan(t, d, started.ID)
	for _, u := range view.Users {
		if u.ID == userID {
			return view, u
		}
	}
	t.Fatalf("user %d not in scan", userID)
	return view, ScannedUser{}
}

func TestWhitelistLetsUserThroughUntilExpiry(t *testing.T) {
	d, _, wl, c := newWhitelistDomain(t)
	ctx := context.Background()

	// User 1 fails the access check.
	scan, u := scanUser(t, d, 1)
	require.Equal(t, UserStatusBad, u.Status)

	added, err := d.AddToWhitelist(ctx, testChannelID, scan.ID, []int64{1}, testAdminID)
	require.NoError(t, err)
	require.Len(t, added, 1)
	assert.Equal(t, c.t.Add(30*24*time.Hour), added[0].ExpiresAt)
	assert.Equal(t, testAdminID, added[0].ApprovedBy)
	assert.Contains(t, wl.entries[testChannelID], int64(1), "entry is persisted")

	// Next scan: passes, is protected from kicks, shows the expiry.
	scan, u = scanUser(t, d, 1)
	assert.Equal(t, UserStatusGood, u.Status)
	assert.True(t, u.Protected)
	require.NotNil(t, u.WhitelistedUntil)
	res, err := d.KickScannedUsers(ctx, testChannelID, scan.ID, []int64{1}, testAdminID)
	require.NoError(t, err)
	assert.Contains(t, res[0].Error, "protected")

	// Automatic scans honour it too.
	users, err := d.ListChannelUsers(ctx, testChannelID)
	require.NoError(t, err)
	assert.Contains(t, ids(users.Good), int64(1))

	// 30 days later the approval has run out: back to the regular check.
	c.t = c.t.Add(30*24*time.Hour + time.Minute)
	_, u = scanUser(t, d, 1)
	assert.Equal(t, UserStatusBad, u.Status)
	assert.Nil(t, u.WhitelistedUntil)

	list, err := d.ListWhitelist(ctx, testChannelID)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.False(t, list[0].Active, "expired entries stay listed for re-approval")
}

func TestWhitelistRenewAndRemove(t *testing.T) {
	d, bot, wl, c := newWhitelistDomain(t)
	ctx := context.Background()
	scan, _ := scanUser(t, d, 1)
	_, err := d.AddToWhitelist(ctx, testChannelID, scan.ID, []int64{1}, testAdminID)
	require.NoError(t, err)

	c.t = c.t.Add(40 * 24 * time.Hour)
	renewed, err := d.RenewWhitelistEntry(ctx, testChannelID, 1, 77)
	require.NoError(t, err)
	assert.True(t, renewed.Active)
	assert.Equal(t, int64(77), renewed.ApprovedBy)
	assert.Equal(t, c.t.Add(30*24*time.Hour), wl.entries[testChannelID][1].ExpiresAt)

	require.NoError(t, d.RemoveWhitelistEntry(ctx, testChannelID, 1, 77))
	assert.Empty(t, wl.entries[testChannelID])
	_, err = d.RenewWhitelistEntry(ctx, testChannelID, 1, 77)
	assert.ErrorIs(t, err, ErrNotWhitelisted)

	// Every change leaves an audit message in the control chat.
	var audit []string
	for _, m := range bot.sent {
		if strings.Contains(m.Text, "Whitelist of") {
			audit = append(audit, m.Text)
		}
	}
	require.Len(t, audit, 3)
	assert.Contains(t, audit[0], "added")
	assert.Contains(t, audit[1], "re-approved")
	assert.Contains(t, audit[2], "removed")
}

func TestWhitelistRejectsUsersOutsideScanAndAdmins(t *testing.T) {
	d, _, _, _ := newWhitelistDomain(t)
	scan, _ := scanUser(t, d, 1)

	_, err := d.AddToWhitelist(context.Background(), testChannelID, scan.ID, []int64{12345}, testAdminID)
	assert.Error(t, err)
	_, err = d.AddToWhitelist(context.Background(), testChannelID, scan.ID, []int64{testAdminID}, testAdminID)
	assert.Error(t, err)
	_, err = d.AddToWhitelist(context.Background(), testChannelID, "nope", []int64{1}, testAdminID)
	assert.ErrorIs(t, err, ErrScanNotFound)
}

func TestWhitelistRemindersSentOnce(t *testing.T) {
	d, bot, wl, c := newWhitelistDomain(t)
	ctx := context.Background()
	scan, _ := scanUser(t, d, 1)
	_, err := d.AddToWhitelist(ctx, testChannelID, scan.ID, []int64{1}, testAdminID)
	require.NoError(t, err)
	reminders := func() []string {
		var out []string
		for _, m := range bot.sent {
			if strings.Contains(m.Text, "runs out soon") || strings.Contains(m.Text, "expired") {
				out = append(out, m.Text)
			}
		}
		return out
	}

	d.remindWhitelists(ctx)
	assert.Empty(t, reminders(), "nothing to remind right after approval")

	c.t = c.t.Add(28 * 24 * time.Hour) // 2 days left
	d.remindWhitelists(ctx)
	d.remindWhitelists(ctx)
	require.Len(t, reminders(), 1, "one reminder per approval period")
	assert.Contains(t, reminders()[0], "runs out soon")

	c.t = c.t.Add(3 * 24 * time.Hour) // expired
	d.remindWhitelists(ctx)
	d.remindWhitelists(ctx)
	require.Len(t, reminders(), 2)
	assert.Contains(t, reminders()[1], "expired")
	assert.True(t, wl.entries[testChannelID][1].ExpiryNotified)

	// Re-approval starts a new period with its own reminder.
	_, err = d.RenewWhitelistEntry(ctx, testChannelID, 1, testAdminID)
	require.NoError(t, err)
	c.t = c.t.Add(29 * 24 * time.Hour)
	d.remindWhitelists(ctx)
	assert.Len(t, reminders(), 3)
}

func TestWhitelistDisabledWithoutStorage(t *testing.T) {
	d, _, _, _, _ := newMiniAppDomain(t, true)
	_, err := d.ListWhitelist(context.Background(), testChannelID)
	assert.ErrorIs(t, err, ErrWhitelistDisabled)
	view, err := d.GetChannel(context.Background(), testChannelID)
	require.NoError(t, err)
	assert.False(t, view.WhitelistEnabled)
}

func ids(users []guard.User) []int64 {
	out := make([]int64, 0, len(users))
	for _, u := range users {
		out = append(out, u.ID)
	}
	return out
}
