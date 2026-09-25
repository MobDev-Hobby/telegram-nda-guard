package scanner

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	guard "github.com/MobDev-Hobby/telegram-nda-guard"
	"github.com/MobDev-Hobby/telegram-nda-guard/processors"
	"github.com/MobDev-Hobby/telegram-nda-guard/storage/channels"
)

const (
	testChannelID = int64(-1001234)
	testBotID     = int64(900)
	testAdminID   = int64(10)
)

type fakeBot struct {
	mu     sync.Mutex
	admins []int64
	sent   []guard.Message
}

func (f *fakeBot) Run(context.Context) error { return nil }
func (f *fakeBot) GetChat(context.Context, int64) (*guard.ChannelInfo, error) {
	return nil, errors.New("not used")
}
func (f *fakeBot) UserID() int64                                        { return testBotID }
func (f *fakeBot) Username() string                                     { return "guardbot" }
func (f *fakeBot) GetInviteLink(context.Context, int64) (string, error) { return "", nil }
func (f *fakeBot) RegisterHandler(context.Context, func(*guard.Update) bool, func(context.Context, *guard.Update)) string {
	return ""
}
func (f *fakeBot) ClearHandler(string) {}
func (f *fakeBot) SendMessage(_ context.Context, m *guard.Message) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, *m)
	return nil
}
func (f *fakeBot) CheckAccessUser(context.Context, int64, int64, ...guard.Permission) (bool, map[guard.Permission]bool, error) {
	return true, nil, nil
}
func (f *fakeBot) PromoteUser(context.Context, int64, int64, ...guard.Permission) (bool, error) {
	return false, nil
}
func (f *fakeBot) GetChatAdministrators(context.Context, int64) ([]int64, error) {
	return f.admins, nil
}
func (f *fakeBot) CallbackResponse(context.Context, guard.CallbackResponse) {}

type fakeUserBot struct {
	users       []guard.User
	invalidated []int64
}

func (f *fakeUserBot) Run(context.Context) error { return nil }
func (f *fakeUserBot) GetChannelUsers(context.Context, int64) ([]guard.User, error) {
	return f.users, nil
}
func (f *fakeUserBot) UserID() int64    { return testBotID }
func (f *fakeUserBot) Username() string { return "guardbot" }
func (f *fakeUserBot) ChannelStats(int64) (guard.ScanStats, bool) {
	return guard.ScanStats{Fetched: len(f.users), Total: 50}, true
}
func (f *fakeUserBot) InvalidateChannel(id int64) { f.invalidated = append(f.invalidated, id) }

// idChecker allows even user IDs, denies odd ones and fails for ID 99.
type idChecker struct{}

func (idChecker) HasAccess(_ context.Context, u *guard.User) (bool, error) {
	if u.ID == 99 {
		return false, errors.New("ap-search timeout")
	}
	return u.ID%2 == 0, nil
}

type fakeKicker struct {
	kicked [][]int64
	opts   []*processors.CleanOptions
}

func (f *fakeKicker) KickUsers(_ context.Context, _ guard.ChannelInfo, users []guard.User, opts *processors.CleanOptions) []processors.KickResult {
	ids := make([]int64, 0, len(users))
	out := make([]processors.KickResult, 0, len(users))
	for _, u := range users {
		ids = append(ids, u.ID)
		out = append(out, processors.KickResult{UserID: u.ID, OK: true})
	}
	f.kicked = append(f.kicked, ids)
	f.opts = append(f.opts, opts)
	return out
}

type memStorage struct {
	records map[int64]channels.ProtectedChannel
}

func (m *memStorage) LoadAll(context.Context) ([]channels.ProtectedChannel, error) { return nil, nil }
func (m *memStorage) Store(_ context.Context, pc *channels.ProtectedChannel) error {
	m.records[pc.ID] = *pc
	return nil
}
func (m *memStorage) Drop(_ context.Context, id int64) error {
	delete(m.records, id)
	return nil
}

type noopProcessor struct{}

func (noopProcessor) ProcessReport(context.Context, processors.AccessReport) {}

func newMiniAppDomain(t *testing.T, allowClean bool) (*Domain, *fakeBot, *fakeUserBot, *fakeKicker, *memStorage) {
	t.Helper()
	bot := &fakeBot{admins: []int64{testAdminID, testBotID}}
	userBot := &fakeUserBot{users: []guard.User{
		{ID: 1, FirstName: "Outsider"},
		{ID: 2, FirstName: "Employee"},
		{ID: testAdminID + 1, FirstName: "Odd"},
		{ID: testAdminID, FirstName: "Admin"},
		{ID: testBotID, FirstName: "Bot"},
		{ID: 99, FirstName: "Unknown"},
	}}
	kicker := &fakeKicker{}
	storage := &memStorage{records: map[int64]channels.ProtectedChannel{}}
	d := New(bot, userBot,
		WithDefaultAccessChecker(idChecker{}),
		WithDefaultScanProcessor(noopProcessor{}),
		WithDefaultCleanProcessor(noopProcessor{}),
		WithUserKicker(kicker),
		WithStorage(storage),
		WithDefaultCleanOptions(processors.CleanOptions{CleanMessages: true}),
	)
	require.NoError(t, d.AddDefaultProtectedChannel(&ProtectedChannel{
		ID:                testChannelID,
		CommandChannelIDs: []int64{555},
		AllowClean:        allowClean,
	}))
	d.channelsMutex.Lock()
	ch := d.channels[testChannelID]
	ch.title, ch.chatType, ch.botOnChannel, ch.botCanClean = "News", guard.ChatTypeChannel, true, true
	d.channels[testChannelID] = ch
	d.channelsMutex.Unlock()
	return d, bot, userBot, kicker, storage
}

func waitScan(t *testing.T, d *Domain, scanID string) ScanView {
	t.Helper()
	var view ScanView
	require.Eventually(t, func() bool {
		var err error
		view, err = d.GetChannelScan(context.Background(), testChannelID, scanID)
		require.NoError(t, err)
		return view.State != ScanStateRunning
	}, 2*time.Second, 5*time.Millisecond)
	return view
}

func TestScanClassifiesAndProtects(t *testing.T) {
	d, _, _, _, _ := newMiniAppDomain(t, true)

	started, err := d.StartChannelScan(context.Background(), testChannelID, testAdminID)
	require.NoError(t, err)
	view := waitScan(t, d, started.ID)

	require.Equal(t, ScanStateDone, view.State)
	assert.True(t, view.Partial)
	assert.Equal(t, guard.ScanStats{Fetched: 6, Total: 50}, view.Stats)

	byID := map[int64]ScannedUser{}
	for _, u := range view.Users {
		byID[u.ID] = u
	}
	assert.Equal(t, UserStatusBad, byID[1].Status)
	assert.Equal(t, UserStatusGood, byID[2].Status)
	assert.Equal(t, UserStatusUnknown, byID[99].Status)
	assert.True(t, byID[testAdminID].Protected)
	assert.True(t, byID[testBotID].Protected)
	assert.False(t, byID[1].Protected)
	// Bad first, so the most likely kicks are on top.
	assert.Equal(t, UserStatusBad, view.Users[0].Status)
}

func TestKickOnlyScannedUnprotectedUsers(t *testing.T) {
	d, bot, userBot, kicker, _ := newMiniAppDomain(t, true)
	started, err := d.StartChannelScan(context.Background(), testChannelID, testAdminID)
	require.NoError(t, err)
	waitScan(t, d, started.ID)

	results, err := d.KickScannedUsers(context.Background(), testChannelID, started.ID,
		[]int64{1, 1, testAdminID, 12345, 99}, testAdminID)
	require.NoError(t, err)

	assert.Equal(t, [][]int64{{1, 99}}, kicker.kicked)
	errs := map[int64]string{}
	for _, r := range results {
		if !r.OK {
			errs[r.UserID] = r.Error
		}
	}
	assert.Contains(t, errs[testAdminID], "protected")
	assert.Contains(t, errs[12345], "not found")
	assert.Equal(t, []int64{testChannelID}, userBot.invalidated)

	// Kicked users are marked, and a second kick of them is refused.
	view, err := d.GetChannelScan(context.Background(), testChannelID, started.ID)
	require.NoError(t, err)
	for _, u := range view.Users {
		if u.ID == 1 {
			assert.Equal(t, UserStatusKicked, u.Status)
		}
	}
	again, err := d.KickScannedUsers(context.Background(), testChannelID, started.ID, []int64{1}, testAdminID)
	require.NoError(t, err)
	assert.Equal(t, "already kicked", again[0].Error)

	require.NotEmpty(t, bot.sent)
	assert.Equal(t, int64(555), bot.sent[0].ChatID)
	assert.Contains(t, bot.sent[0].Text, "Removed <b>2/2</b>")
}

func TestKickRefusedWhenManualCleanDisabled(t *testing.T) {
	d, _, _, kicker, _ := newMiniAppDomain(t, false)
	started, err := d.StartChannelScan(context.Background(), testChannelID, testAdminID)
	require.NoError(t, err)
	waitScan(t, d, started.ID)

	_, err = d.KickScannedUsers(context.Background(), testChannelID, started.ID, []int64{1}, testAdminID)
	assert.ErrorIs(t, err, ErrManualCleanDisabled)
	assert.Empty(t, kicker.kicked)
}

func TestKickRejectsScanOfAnotherChannel(t *testing.T) {
	d, _, _, _, _ := newMiniAppDomain(t, true)
	started, err := d.StartChannelScan(context.Background(), testChannelID, testAdminID)
	require.NoError(t, err)
	waitScan(t, d, started.ID)

	require.NoError(t, d.AddDefaultProtectedChannel(&ProtectedChannel{ID: -100777, CommandChannelIDs: []int64{1}, AllowClean: true}))
	d.channelsMutex.Lock()
	ch := d.channels[-100777]
	ch.botOnChannel, ch.botCanClean = true, true
	d.channels[-100777] = ch
	d.channelsMutex.Unlock()

	_, err = d.KickScannedUsers(context.Background(), -100777, started.ID, []int64{1}, testAdminID)
	assert.ErrorIs(t, err, ErrScanNotFound)
}

func TestSetChannelSettingsPersistsCleanOptions(t *testing.T) {
	d, _, _, kicker, storage := newMiniAppDomain(t, true)

	view, err := d.GetChannel(context.Background(), testChannelID)
	require.NoError(t, err)
	assert.False(t, view.CustomCleanOptions)
	assert.True(t, view.CleanMessages, "defaults are shown until the channel overrides them")

	require.NoError(t, d.SetChannelSettings(context.Background(), testChannelID, ChannelSettings{
		AutoScan: true, AllowClean: true, KeepBanned: true, CleanUnknown: true,
	}))

	rec := storage.records[testChannelID]
	require.NotNil(t, rec.CleanOptions)
	assert.Equal(t, processors.CleanOptions{KeepBanned: true, CleanUnknown: true}, *rec.CleanOptions)
	assert.True(t, rec.AutoScan)
	assert.True(t, d.channelHasTicker(testChannelID))

	view, err = d.GetChannel(context.Background(), testChannelID)
	require.NoError(t, err)
	assert.True(t, view.CustomCleanOptions)
	assert.True(t, view.KeepBanned)
	assert.False(t, view.CleanMessages)

	// Manual kicks use the channel's options.
	started, err := d.StartChannelScan(context.Background(), testChannelID, testAdminID)
	require.NoError(t, err)
	waitScan(t, d, started.ID)
	_, err = d.KickScannedUsers(context.Background(), testChannelID, started.ID, []int64{1}, testAdminID)
	require.NoError(t, err)
	require.Len(t, kicker.opts, 1)
	assert.True(t, kicker.opts[0].KeepBanned)
}

func TestAppHandlerButtons(t *testing.T) {
	cases := []struct {
		name, chatType, shortName string
		wantWebApp, wantURL       string
	}{
		{"private chat opens web app", guard.ChatTypePrivate, "", "https://guard.example/miniapp/", ""},
		{"group uses direct link", guard.ChatTypeSupergroup, "nda", "", "https://t.me/guardbot/nda"},
		{"group without short name points to private chat", guard.ChatTypeGroup, "", "", "https://t.me/guardbot"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			bot := &fakeBot{}
			d := New(bot, &fakeUserBot{}, WithMiniApp("https://guard.example/miniapp/", c.shortName))
			d.AppHandler(context.Background(), &guard.Update{Message: &guard.MessageReceived{
				Message: guard.Message{ChatID: 1, ChatType: c.chatType, Text: "/app"},
			}})
			require.Len(t, bot.sent, 1)
			button := bot.sent[0].InlineButtons[0][0]
			assert.Equal(t, c.wantWebApp, button.WebAppURL)
			assert.Equal(t, c.wantURL, button.URL)
		})
	}
}
