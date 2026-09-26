package scanner

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	guard "github.com/MobDev-Hobby/telegram-nda-guard"
	"github.com/MobDev-Hobby/telegram-nda-guard/processors"
	"github.com/MobDev-Hobby/telegram-nda-guard/storage/audit"
	"github.com/MobDev-Hobby/telegram-nda-guard/storage/whitelist"
)

type memWhitelist struct {
	entries map[int64]map[int64]whitelist.Entry
}

func (m *memWhitelist) LoadWhitelist(_ context.Context, channelID int64) ([]whitelist.Entry, error) {
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

type memAudit struct {
	mu     sync.Mutex
	events []audit.Event
}

func (m *memAudit) AppendAudit(_ context.Context, _ int64, e audit.Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append([]audit.Event{e}, m.events...)
	return nil
}
func (m *memAudit) ListAudit(_ context.Context, _ int64, limit int) ([]audit.Event, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.events) > limit {
		return m.events[:limit], nil
	}
	return m.events, nil
}
func (m *memAudit) actions() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []string{}
	for i := len(m.events) - 1; i >= 0; i-- {
		out = append(out, m.events[i].Action)
	}
	return out
}

type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}
func (c *clock) add(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

type whitelistEnv struct {
	d     *Domain
	bot   *fakeBot
	wl    *memWhitelist
	audit *memAudit
	clock *clock
}

func newWhitelistDomain(t *testing.T) whitelistEnv {
	t.Helper()
	d, bot, _, _, _ := newMiniAppDomain(t, true)
	env := whitelistEnv{
		d:     d,
		bot:   bot,
		wl:    &memWhitelist{entries: map[int64]map[int64]whitelist.Entry{}},
		audit: &memAudit{},
		clock: &clock{t: time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)},
	}
	d.whitelistStorage = env.wl
	d.auditStorage = env.audit
	d.now = env.clock.now
	return env
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

func (e whitelistEnv) sentTexts(substr string) []string {
	e.bot.mu.Lock()
	defer e.bot.mu.Unlock()
	var out []string
	for _, m := range e.bot.sent {
		if strings.Contains(m.Text, substr) {
			out = append(out, m.Text)
		}
	}
	return out
}

func (e whitelistEnv) sentTo(chatID int64, substr string) int {
	e.bot.mu.Lock()
	defer e.bot.mu.Unlock()
	n := 0
	for _, m := range e.bot.sent {
		if m.ChatID == chatID && strings.Contains(m.Text, substr) {
			n++
		}
	}
	return n
}

func TestWhitelistProtectsEvenAfterExpiry(t *testing.T) {
	env := newWhitelistDomain(t)
	d, ctx := env.d, context.Background()

	scan, u := scanUser(t, d, 1)
	require.Equal(t, UserStatusBad, u.Status)

	added, err := d.AddToWhitelist(ctx, testChannelID, scan.ID, []int64{1}, "  contractor, NDA signed  ", 0, testAdminID)
	require.NoError(t, err)
	require.Len(t, added, 1)
	assert.Equal(t, env.clock.now().Add(30*24*time.Hour), added[0].ExpiresAt)
	assert.Equal(t, "contractor, NDA signed", added[0].Note, "note is trimmed and stored")
	assert.Equal(t, "contractor, NDA signed", env.wl.entries[testChannelID][1].Note)

	_, u = scanUser(t, d, 1)
	assert.Equal(t, UserStatusWhitelisted, u.Status)
	assert.True(t, u.Protected)
	require.NotNil(t, u.Whitelist)
	assert.False(t, u.Whitelist.Expired)

	// Past the term the user is still protected — only flagged for review.
	env.clock.add(31 * 24 * time.Hour)
	scan, u = scanUser(t, d, 1)
	assert.Equal(t, UserStatusWhitelisted, u.Status)
	require.NotNil(t, u.Whitelist)
	assert.True(t, u.Whitelist.Expired)
	res, err := d.KickScannedUsers(ctx, testChannelID, scan.ID, []int64{1}, testAdminID)
	require.NoError(t, err)
	assert.Contains(t, res[0].Error, "protected")

	users, err := d.ListChannelUsers(ctx, testChannelID)
	require.NoError(t, err)
	assert.Contains(t, ids(users.Good), int64(1), "automatic clean does not touch overdue entries either")

	list, err := d.ListWhitelist(ctx, testChannelID)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.True(t, list[0].Expired)
}

func TestWhitelistNoteTooLong(t *testing.T) {
	env := newWhitelistDomain(t)
	scan, _ := scanUser(t, env.d, 1)
	_, err := env.d.AddToWhitelist(context.Background(), testChannelID, scan.ID, []int64{1}, strings.Repeat("я", MaxWhitelistNoteLen+1), 0, testAdminID)
	assert.ErrorIs(t, err, ErrNoteTooLong)
	assert.Empty(t, env.wl.entries[testChannelID])
}

func TestWhitelistRenewAndRemove(t *testing.T) {
	env := newWhitelistDomain(t)
	d, ctx := env.d, context.Background()
	scan, _ := scanUser(t, d, 1)
	_, err := d.AddToWhitelist(ctx, testChannelID, scan.ID, []int64{1}, "first", 0, testAdminID)
	require.NoError(t, err)

	env.clock.add(40 * 24 * time.Hour)
	renewed, err := d.RenewWhitelistEntry(ctx, testChannelID, 1, "", 0, 77)
	require.NoError(t, err)
	assert.False(t, renewed.Expired)
	assert.Equal(t, int64(77), renewed.ApprovedBy)
	assert.Equal(t, "first", renewed.Note, "empty note keeps the old one")
	renewed, err = d.RenewWhitelistEntry(ctx, testChannelID, 1, "reviewed again", 0, 77)
	require.NoError(t, err)
	assert.Equal(t, "reviewed again", renewed.Note)

	require.NoError(t, d.RemoveWhitelistEntry(ctx, testChannelID, 1, 77))
	assert.Empty(t, env.wl.entries[testChannelID])
	_, err = d.RenewWhitelistEntry(ctx, testChannelID, 1, "", 0, 77)
	assert.ErrorIs(t, err, ErrNotWhitelisted)

	assert.Equal(t, []string{
		audit.ActionScanCompleted, audit.ActionWhitelistAdded, audit.ActionWhitelistRenewed,
		audit.ActionWhitelistRenewed, audit.ActionWhitelistRemoved,
	}, env.audit.actions())
	assert.Len(t, env.sentTexts("Whitelist of"), 4, "every change is also posted to the control chat")
}

func TestWhitelistRejectsUsersOutsideScanAndAdmins(t *testing.T) {
	env := newWhitelistDomain(t)
	scan, _ := scanUser(t, env.d, 1)
	ctx := context.Background()

	_, err := env.d.AddToWhitelist(ctx, testChannelID, scan.ID, []int64{12345}, "", 0, testAdminID)
	assert.Error(t, err)
	_, err = env.d.AddToWhitelist(ctx, testChannelID, scan.ID, []int64{1, testAdminID}, "", 0, testAdminID)
	assert.Error(t, err)
	assert.Empty(t, env.wl.entries[testChannelID], "a bad batch writes nothing")
	_, err = env.d.AddToWhitelist(ctx, testChannelID, "nope", []int64{1}, "", 0, testAdminID)
	assert.ErrorIs(t, err, ErrScanNotFound)
}

func TestWhitelistRemindersDailyAfterExpiryToManagers(t *testing.T) {
	env := newWhitelistDomain(t)
	d, ctx := env.d, context.Background()
	require.NoError(t, d.JoinChannel(ctx, testChannelID, testAdminID))
	scan, _ := scanUser(t, d, 1)
	_, err := d.AddToWhitelist(ctx, testChannelID, scan.ID, []int64{1}, "", 0, testAdminID)
	require.NoError(t, err)

	d.remindWhitelists(ctx)
	assert.Empty(t, env.sentTexts("needs a review"), "nothing to remind right after approval")

	env.clock.add(28 * 24 * time.Hour) // 2 days left
	d.remindWhitelists(ctx)
	d.remindWhitelists(ctx)
	require.Len(t, env.sentTexts("runs out soon"), 2, "one reminder per period, to the control chat and the manager")
	assert.Equal(t, 1, env.sentTo(testAdminID, "runs out soon"), "managers get it in private")

	env.clock.add(3 * 24 * time.Hour) // expired
	d.remindWhitelists(ctx)
	d.remindWhitelists(ctx)
	assert.Equal(t, 1, env.sentTo(555, "still protected"))

	env.clock.add(12 * time.Hour)
	d.remindWhitelists(ctx)
	assert.Equal(t, 1, env.sentTo(555, "still protected"), "at most once a day")

	env.clock.add(12 * time.Hour)
	d.remindWhitelists(ctx)
	assert.Equal(t, 2, env.sentTo(555, "still protected"), "and again the next day")
	assert.Equal(t, 2, env.sentTo(testAdminID, "still protected"))

	expiredEvents := 0
	for _, a := range env.audit.actions() {
		if a == audit.ActionWhitelistExpired {
			expiredEvents++
		}
	}
	assert.Equal(t, 1, expiredEvents, "expiry is logged once, not every day")

	// Renewal ends the daily reminders.
	_, err = d.RenewWhitelistEntry(ctx, testChannelID, 1, "", 0, testAdminID)
	require.NoError(t, err)
	env.clock.add(24 * time.Hour)
	d.remindWhitelists(ctx)
	assert.Equal(t, 2, env.sentTo(555, "still protected"))
}

func TestRecheckBypassesCacheAndWhitelist(t *testing.T) {
	env := newWhitelistDomain(t)
	d, ctx := env.d, context.Background()
	checker := &countingChecker{verdict: map[int64]bool{1: false}}
	d.defaultAccessChecker = checker
	d.channelsMutex.Lock()
	pc := d.protectedChannels[testChannelID]
	pc.AccessChecker = checker
	d.protectedChannels[testChannelID] = pc
	d.channelsMutex.Unlock()

	scan, u := scanUser(t, d, 1)
	require.Equal(t, UserStatusBad, u.Status)

	// The user got access meanwhile: a recheck must show it.
	checker.set(1, true)
	got, err := d.RecheckScannedUser(ctx, testChannelID, scan.ID, 1, testAdminID)
	require.NoError(t, err)
	assert.Equal(t, UserStatusGood, got.Status)
	assert.Equal(t, UserStatusGood, got.Check)
	assert.Equal(t, 1, checker.invalidated[1], "the cache is dropped before asking again")

	// Whitelisted users can be rechecked too; they stay whitelisted.
	_, err = d.AddToWhitelist(ctx, testChannelID, scan.ID, []int64{2}, "", 0, testAdminID)
	require.NoError(t, err)
	checker.set(2, false)
	got, err = d.RecheckScannedUser(ctx, testChannelID, scan.ID, 2, testAdminID)
	require.NoError(t, err)
	assert.Equal(t, UserStatusWhitelisted, got.Status)
	assert.Equal(t, UserStatusBad, got.Check, "shows what the checker says without the whitelist")

	assert.Contains(t, env.audit.actions(), audit.ActionUserRechecked)
	_, err = d.RecheckScannedUser(ctx, testChannelID, scan.ID, 424242, testAdminID)
	assert.Error(t, err)
}

func TestHealthTrafficLight(t *testing.T) {
	now := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name   string
		last   *processors.CheckSummary
		status string
		reason string
	}{
		{"never checked", nil, HealthYellow, HealthReasonNeverCheck},
		{"fresh and clean", &processors.CheckSummary{At: now.Add(-time.Hour)}, HealthGreen, HealthReasonOK},
		{"violations", &processors.CheckSummary{At: now.Add(-time.Hour), Bad: 2}, HealthRed, HealthReasonViolations},
		{"4 days old", &processors.CheckSummary{At: now.Add(-4 * 24 * time.Hour)}, HealthYellow, HealthReasonStale},
		{"8 days old", &processors.CheckSummary{At: now.Add(-8 * 24 * time.Hour)}, HealthRed, HealthReasonVeryStale},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := channelHealth(c.last, now)
			assert.Equal(t, c.status, h.Status)
			assert.Equal(t, c.reason, h.Reason)
		})
	}
}

func TestScansFeedHealthAndKicksClearViolations(t *testing.T) {
	env := newWhitelistDomain(t)
	d, ctx := env.d, context.Background()

	view, err := d.GetChannel(ctx, testChannelID)
	require.NoError(t, err)
	assert.Equal(t, HealthYellow, view.Health.Status)

	scan, _ := scanUser(t, d, 1)
	view, _ = d.GetChannel(ctx, testChannelID)
	assert.Equal(t, HealthRed, view.Health.Status, "the scan found users without access")
	require.NotNil(t, view.Health.LastCheck)
	bad := view.Health.LastCheck.Bad
	require.Positive(t, bad)

	var badIDs []int64
	for _, u := range scan.Users {
		if u.Status == UserStatusBad && !u.Protected {
			badIDs = append(badIDs, u.ID)
		}
	}
	_, err = d.KickScannedUsers(ctx, testChannelID, scan.ID, badIDs, testAdminID)
	require.NoError(t, err)
	view, _ = d.GetChannel(ctx, testChannelID)
	assert.Equal(t, HealthGreen, view.Health.Status, "no violations left after the kick")
	assert.Contains(t, env.audit.actions(), audit.ActionUsersKicked)

	env.clock.add(4 * 24 * time.Hour)
	view, _ = d.GetChannel(ctx, testChannelID)
	assert.Equal(t, HealthYellow, view.Health.Status)
}

func TestBotScanRecordsHealthAndAudit(t *testing.T) {
	env := newWhitelistDomain(t)
	d, ctx := env.d, context.Background()
	ch, _ := d.getChannel(testChannelID)
	pc, _ := d.getProtectedChannel(testChannelID)

	d.ProcessRequest(ctx, ScanRequest{
		requestType:     AutoScan,
		channelInfo:     ch,
		accessChecker:   pc.AccessChecker,
		reportProcessor: noopProcessor{},
	})

	pc, _ = d.getProtectedChannel(testChannelID)
	require.NotNil(t, pc.LastCheck)
	assert.Positive(t, pc.LastCheck.Bad)
	require.NotEmpty(t, env.audit.events)
	assert.Equal(t, audit.ActionScanCompleted, env.audit.events[0].Action)
	assert.Equal(t, "schedule", env.audit.events[0].Details["source"])
}

func TestJoinChannelMakesManager(t *testing.T) {
	env := newWhitelistDomain(t)
	d, ctx := env.d, context.Background()
	d.NoteUserName(testAdminID, "Anna Admin")

	assert.False(t, d.IsChannelManager(testChannelID, testAdminID))
	require.NoError(t, d.JoinChannel(ctx, testChannelID, testAdminID))
	require.NoError(t, d.JoinChannel(ctx, testChannelID, testAdminID), "joining twice is harmless")
	assert.True(t, d.IsChannelManager(testChannelID, testAdminID))

	pc, _ := d.getProtectedChannel(testChannelID)
	assert.Equal(t, []int64{testAdminID}, pc.Managers)
	assert.Equal(t, []string{audit.ActionChannelJoined}, env.audit.actions())
	assert.Equal(t, "Anna Admin", env.audit.events[0].ActorName)
	assert.Len(t, env.sentTexts("joined"), 1)
}

func TestCanUseMiniAppOnlyForCheckerPassers(t *testing.T) {
	env := newWhitelistDomain(t)
	ctx := context.Background()
	ok, err := env.d.CanUseMiniApp(ctx, guard.User{ID: 2, FirstName: "Emp"})
	require.NoError(t, err)
	assert.True(t, ok)
	ok, err = env.d.CanUseMiniApp(ctx, guard.User{ID: 1})
	require.NoError(t, err)
	assert.False(t, ok)
	_, err = env.d.CanUseMiniApp(ctx, guard.User{ID: 99})
	assert.Error(t, err, "a failed check is not a pass")
}

func TestWhitelistDisabledWithoutStorage(t *testing.T) {
	d, _, _, _, _ := newMiniAppDomain(t, true)
	_, err := d.ListWhitelist(context.Background(), testChannelID)
	assert.ErrorIs(t, err, ErrWhitelistDisabled)
	view, err := d.GetChannel(context.Background(), testChannelID)
	require.NoError(t, err)
	assert.False(t, view.WhitelistEnabled)
	events, err := d.ListAudit(context.Background(), testChannelID, 10)
	require.NoError(t, err)
	assert.Empty(t, events)
}

// countingChecker answers from a verdict map and records invalidations.
type countingChecker struct {
	mu          sync.Mutex
	verdict     map[int64]bool
	invalidated map[int64]int
}

func (c *countingChecker) set(id int64, ok bool) {
	c.mu.Lock()
	c.verdict[id] = ok
	c.mu.Unlock()
}
func (c *countingChecker) HasAccess(_ context.Context, u *guard.User) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.verdict[u.ID], nil
}
func (c *countingChecker) Invalidate(id int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.invalidated == nil {
		c.invalidated = map[int64]int{}
	}
	c.invalidated[id]++
}

func ids(users []guard.User) []int64 {
	out := make([]int64, 0, len(users))
	for _, u := range users {
		out = append(out, u.ID)
	}
	return out
}

func TestKickFromOlderScanSkipsUsersWhitelistedSince(t *testing.T) {
	env := newWhitelistDomain(t)
	d, ctx := env.d, context.Background()

	older, u := scanUser(t, d, 1)
	require.False(t, u.Protected)
	// Another admin whitelists the user from a newer scan.
	newer, _ := scanUser(t, d, 1)
	_, err := d.AddToWhitelist(ctx, testChannelID, newer.ID, []int64{1}, "", 0, testAdminID)
	require.NoError(t, err)

	results, err := d.KickScannedUsers(ctx, testChannelID, older.ID, []int64{1}, testAdminID)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.False(t, results[0].OK)
	assert.Contains(t, results[0].Error, "whitelisted")
}
