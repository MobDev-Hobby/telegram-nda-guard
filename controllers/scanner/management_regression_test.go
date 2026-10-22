package scanner

import (
	"context"
	"sync"
	"testing"
	"time"

	guard "github.com/MobDev-Hobby/telegram-nda-guard"
	"github.com/MobDev-Hobby/telegram-nda-guard/processors"
	"github.com/MobDev-Hobby/telegram-nda-guard/storage/channels"
	"github.com/MobDev-Hobby/telegram-nda-guard/storage/knownchats"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWhitelistAdminStillCannotBeKicked(t *testing.T) {
	env := newWhitelistDomain(t)
	ctx := context.Background()
	// The administrator fails the employee check, which is exactly when an
	// explicit whitelist exception is needed.
	pc, _ := env.d.getProtectedChannel(testChannelID)
	pc.AccessChecker = &countingChecker{verdict: map[int64]bool{}}
	require.NoError(t, env.d.updateProtectedChannel(ctx, testChannelID, func(p *ProtectedChannel) { p.AccessChecker = pc.AccessChecker }))
	scan, admin := scanUser(t, env.d, testAdminID)
	require.Equal(t, UserStatusBad, admin.Status)
	require.True(t, admin.Protected)
	_, err := env.d.AddToWhitelist(ctx, testChannelID, scan.ID, []int64{testAdminID}, "approved admin", 0, testAdminID)
	require.NoError(t, err)
	view, err := env.d.GetChannelScan(ctx, testChannelID, scan.ID)
	require.NoError(t, err)
	for _, user := range view.Users {
		if user.ID == testAdminID {
			assert.Equal(t, UserStatusWhitelisted, user.Status)
			assert.True(t, user.Protected)
		}
	}
	users, err := env.d.ListChannelUsers(ctx, testChannelID)
	require.NoError(t, err)
	for _, user := range users.Bad {
		assert.NotEqual(t, testAdminID, user.ID)
	}
	results, err := env.d.KickScannedUsers(ctx, testChannelID, scan.ID, []int64{testAdminID}, testAdminID)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.False(t, results[0].OK)
	assert.Contains(t, results[0].Error, "protected")
}

func TestKickProtectsAdminPromotedAfterScan(t *testing.T) {
	d, bot, _, kicker, _ := newMiniAppDomain(t, true)
	scan, _ := scanUser(t, d, 1)
	bot.admins = append(bot.admins, 1)
	results, err := d.KickScannedUsers(context.Background(), testChannelID, scan.ID, []int64{1}, testAdminID)
	require.NoError(t, err)
	assert.False(t, results[0].OK)
	require.Len(t, kicker.kicked, 1)
	assert.Empty(t, kicker.kicked[0])
}

func TestCachedScanRemainsVisibleButExpiredActionsAreRefused(t *testing.T) {
	env := newWhitelistDomain(t)
	scan, _ := scanUser(t, env.d, 1)
	env.d.scansMutex.Lock()
	past := time.Now().Add(-2 * time.Hour)
	env.d.scans[scan.ID].view.FinishedAt = &past
	env.d.scansMutex.Unlock()
	latest, err := env.d.GetChannelScan(context.Background(), testChannelID, "latest")
	require.NoError(t, err)
	assert.Equal(t, scan.ID, latest.ID)
	assert.True(t, latest.ReadOnly)
	assert.NotEmpty(t, latest.Users)
	_, err = env.d.KickScannedUsers(context.Background(), testChannelID, scan.ID, []int64{1}, testAdminID)
	assert.ErrorIs(t, err, ErrScanNotFound)
	_, err = env.d.AddToWhitelist(context.Background(), testChannelID, scan.ID, []int64{1}, "", 0, testAdminID)
	assert.ErrorIs(t, err, ErrScanNotFound)
	_, err = env.d.RecheckScannedUser(context.Background(), testChannelID, scan.ID, 1, testAdminID)
	assert.ErrorIs(t, err, ErrScanNotFound)
	_, err = env.d.GetChannelScan(context.Background(), -9876, "latest")
	assert.ErrorIs(t, err, ErrScanNotFound)
}

type capturedReports struct{ reports []processors.AccessReport }

func (p *capturedReports) ProcessReport(_ context.Context, report processors.AccessReport) {
	p.reports = append(p.reports, report)
}

type completeUserBot struct{ *fakeUserBot }

func (f completeUserBot) ChannelStats(int64) (guard.ScanStats, bool) {
	return guard.ScanStats{Fetched: len(f.users), Total: len(f.users)}, true
}

func TestScheduledNotificationsAndCacheDoNotDisableCleaning(t *testing.T) {
	for _, mode := range []string{NotificationAll, NotificationViolationsSummary, NotificationViolationsOnly} {
		t.Run(mode, func(t *testing.T) {
			env := newWhitelistDomain(t)
			ctx := context.Background()
			ub := completeUserBot{&fakeUserBot{users: []guard.User{{ID: 2, FirstName: "Employee"}}}}
			env.d.userBot = ub
			spy := &capturedReports{}
			require.NoError(t, env.d.updateProtectedChannel(ctx, testChannelID, func(pc *ProtectedChannel) {
				pc.Notifications = &processors.NotificationOptions{Mode: mode, IntervalHours: 6}
			}))
			ch, _ := env.d.getChannel(testChannelID)
			req := ScanRequest{requestType: AutoScan, channelInfo: ch, accessChecker: idChecker{}, reportProcessor: spy}
			env.d.ProcessRequest(ctx, req)
			if mode == NotificationAll {
				assert.Equal(t, []int64{555}, spy.reports[0].ReportChannels)
			} else {
				assert.Empty(t, spy.reports[0].ReportChannels)
			}
			// A bot-side scan is now immediately available in the app too.
			latest, err := env.d.GetChannelScan(ctx, testChannelID, "latest")
			require.NoError(t, err)
			require.Len(t, latest.Users, 1)
			assert.Equal(t, int64(2), latest.Users[0].ID)
			assert.NotNil(t, latest.FinishedAt)
			env.clock.add(6 * time.Hour)
			ub.users = []guard.User{{ID: 1, FirstName: "Outsider"}}
			env.d.ProcessRequest(ctx, req)
			assert.Equal(t, []int64{555}, spy.reports[1].ReportChannels)
			assert.Equal(t, mode == NotificationViolationsOnly, spy.reports[1].ViolationsOnly)
			// The cleaner must still execute even when its outgoing summary is muted.
			req.requestType = AutoClean
			env.d.ProcessRequest(ctx, req)
			require.Len(t, spy.reports, 3)
			assert.Len(t, spy.reports[2].DeniedUsers, 1)
			assert.Empty(t, spy.reports[2].ReportChannels)
			pc, _ := env.d.getProtectedChannel(testChannelID)
			assert.Equal(t, env.clock.now(), pc.LastNotifiedAt)
		})
	}
}

func TestNotificationSettingsValidationAndPersistence(t *testing.T) {
	env := newWhitelistDomain(t)
	ctx := context.Background()
	for _, opts := range []processors.NotificationOptions{{Mode: "invalid", IntervalHours: 24}, {Mode: NotificationAll, IntervalHours: 0}, {Mode: NotificationAll, IntervalHours: 169}} {
		err := env.d.SetChannelSettings(ctx, testChannelID, ChannelSettings{Notifications: &opts}, testAdminID)
		assert.Error(t, err)
	}
	opts := processors.NotificationOptions{Mode: NotificationViolationsOnly, IntervalHours: 72}
	require.NoError(t, env.d.SetChannelSettings(ctx, testChannelID, ChannelSettings{Notifications: &opts}, testAdminID))
	// An older client omitting this new field retains the saved preference.
	require.NoError(t, env.d.SetChannelSettings(ctx, testChannelID, ChannelSettings{AllowClean: true}, testAdminID))
	pc, _ := env.d.getProtectedChannel(testChannelID)
	assert.Equal(t, &opts, storageRecord(pc).Notifications)
	view, err := env.d.GetChannel(ctx, testChannelID)
	require.NoError(t, err)
	assert.Equal(t, opts, view.Notifications)
	assert.Empty(t, env.sentTexts("Settings of"), "quiet mode keeps the audit in the app")
}

func TestRightsAlertsIdentifyChatAndActorAndAreThrottled(t *testing.T) {
	env := newWhitelistDomain(t)
	pc, _ := env.d.getProtectedChannel(testChannelID)
	pc.Title, pc.ChatType, pc.AddedBy, pc.AddedByName = "Team <NDA>", guard.ChatTypeSupergroup, 44, "Artem"
	require.NoError(t, env.d.AddDefaultProtectedChannel(&pc))
	// Simulate Telegram failing to return a title: persisted metadata must win.
	env.d.channelsMutex.Lock()
	ch := env.d.channels[testChannelID]
	ch.title = ""
	env.d.channels[testChannelID] = ch
	env.d.channelsMutex.Unlock()
	require.NoError(t, env.d.updateProtectedChannel(context.Background(), testChannelID, func(p *ProtectedChannel) { p.Title, p.AddedBy, p.AddedByName = pc.Title, pc.AddedBy, pc.AddedByName }))
	view, err := env.d.GetChannel(context.Background(), testChannelID)
	require.NoError(t, err)
	assert.Equal(t, "Team <NDA>", view.Title)
	assert.Equal(t, "Artem", view.AddedByName)
	env.d.notifyRightsIssue(context.Background(), pc, "restrict", "Promote the bot")
	env.d.notifyRightsIssue(context.Background(), pc, "restrict", "Promote the bot")
	require.Len(t, env.bot.sent, 1)
	assert.Contains(t, env.bot.sent[0].Text, "ID -1001234")
	assert.Contains(t, env.bot.sent[0].Text, "Artem")
	assert.Contains(t, env.bot.sent[0].Text, "Team &lt;NDA&gt;")
}

type blockingStore struct {
	*memStorage
	once    sync.Once
	entered chan struct{}
	release chan struct{}
}

func (s *blockingStore) Store(ctx context.Context, pc *channels.ProtectedChannel) error {
	s.once.Do(func() { close(s.entered); <-s.release })
	return s.memStorage.Store(ctx, pc)
}

func TestConcurrentAddAndJoinPersistAllManagers(t *testing.T) {
	d, _, _, _, storage := newMiniAppDomain(t, true)
	blocking := &blockingStore{memStorage: storage, entered: make(chan struct{}), release: make(chan struct{})}
	d.storage = blocking
	added := make(chan error, 1)
	go func() {
		added <- d.AddDefaultProtectedChannel(&ProtectedChannel{ID: testChannelID, CommandChannelIDs: []int64{777}, Managers: []int64{77}})
	}()
	<-blocking.entered
	joined := make(chan error, 1)
	go func() { joined <- d.JoinChannel(context.Background(), testChannelID, 88) }()
	close(blocking.release)
	require.NoError(t, <-added)
	require.NoError(t, <-joined)
	assert.ElementsMatch(t, []int64{77, 88}, storage.records[testChannelID].Managers)
	assert.True(t, d.IsChannelManager(testChannelID, 77))
	assert.True(t, d.IsChannelManager(testChannelID, 88))
}

type slowMetadataBot struct {
	*fakeBot
	release chan struct{}
}

func (b slowMetadataBot) GetChat(ctx context.Context, _ int64) (*guard.ChannelInfo, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-b.release:
		return &guard.ChannelInfo{Title: "Resolved", Type: guard.ChatTypeSupergroup}, nil
	}
}

func TestConnectPublishesIdentifiedPendingChannelBeforeTelegramFinishes(t *testing.T) {
	d, bot, _, _, _ := newMiniAppDomain(t, true)
	release := make(chan struct{})
	d.telegramBot = slowMetadataBot{bot, release}
	const id = int64(-9988)
	d.knownChats[id] = knownchats.Chat{ID: id, Title: "New team", Type: guard.ChatTypeSupergroup, AddedBy: 45, AddedByName: "Artem", AddedAt: time.Now()}
	require.NoError(t, d.ConnectChat(context.Background(), id, 55))
	view, err := d.GetChannel(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, "New team", view.Title)
	assert.Equal(t, int64(45), view.AddedBy)
	assert.True(t, view.Connecting)
	assert.True(t, d.IsChannelManager(id, 55))
	close(release)
	require.Eventually(t, func() bool { v, _ := d.GetChannel(context.Background(), id); return !v.Connecting }, time.Second, time.Millisecond)
	view, _ = d.GetChannel(context.Background(), id)
	assert.Equal(t, "Resolved", view.Title)
}
