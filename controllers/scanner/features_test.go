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
	"github.com/MobDev-Hobby/telegram-nda-guard/storage/audit"
	"github.com/MobDev-Hobby/telegram-nda-guard/storage/joinrequests"
	"github.com/MobDev-Hobby/telegram-nda-guard/storage/knownchats"
)

// Join request decisions made through the fake bot.
var (
	joinMu       sync.Mutex
	joinApproved = map[int64][]int64{}
	joinDeclined = map[int64][]int64{}
	joinGone     = map[int64]bool{}
)

func (f *fakeBot) ApproveJoinRequest(_ context.Context, chatID, userID int64) error {
	joinMu.Lock()
	defer joinMu.Unlock()
	if joinGone[userID] {
		return errors.New("Bad Request: HIDE_REQUESTER_MISSING")
	}
	joinApproved[chatID] = append(joinApproved[chatID], userID)
	return nil
}

func (f *fakeBot) DeclineJoinRequest(_ context.Context, chatID, userID int64) error {
	joinMu.Lock()
	defer joinMu.Unlock()
	joinDeclined[chatID] = append(joinDeclined[chatID], userID)
	return nil
}

func resetJoins() {
	joinMu.Lock()
	joinApproved, joinDeclined, joinGone = map[int64][]int64{}, map[int64][]int64{}, map[int64]bool{}
	joinMu.Unlock()
}

type memJoinRequests struct {
	mu   sync.Mutex
	reqs map[int64]map[int64]joinrequests.Request
}

func (m *memJoinRequests) LoadJoinRequests(_ context.Context, chatID int64) ([]joinrequests.Request, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []joinrequests.Request{}
	for _, r := range m.reqs[chatID] {
		out = append(out, r)
	}
	return out, nil
}
func (m *memJoinRequests) StoreJoinRequest(_ context.Context, chatID int64, r joinrequests.Request) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.reqs[chatID] == nil {
		m.reqs[chatID] = map[int64]joinrequests.Request{}
	}
	m.reqs[chatID][r.UserID] = r
	return nil
}
func (m *memJoinRequests) DropJoinRequest(_ context.Context, chatID, userID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.reqs[chatID], userID)
	return nil
}

func joinUpdate(userID int64, username string) *guard.Update {
	return &guard.Update{JoinRequest: &guard.JoinRequest{
		Chat: guard.ChannelInfo{ID: testChannelID, Type: guard.ChatTypeChannel},
		User: guard.User{ID: userID, FirstName: "U", Username: username},
		At:   time.Date(2026, 9, 25, 11, 0, 0, 0, time.UTC).Unix(),
	}}
}

func setJoinMode(t *testing.T, d *Domain, mode string) {
	t.Helper()
	require.NoError(t, d.SetChannelSettings(context.Background(), testChannelID, ChannelSettings{AllowClean: true, JoinRequests: mode}, testAdminID))
}

func newJoinEnv(t *testing.T) (whitelistEnv, *memJoinRequests, *countingChecker) {
	t.Helper()
	resetJoins()
	env := newWhitelistDomain(t)
	store := &memJoinRequests{reqs: map[int64]map[int64]joinrequests.Request{}}
	env.d.joinRequestStorage = store
	checker := &countingChecker{verdict: map[int64]bool{2: true}}
	env.d.defaultAccessChecker = checker
	env.d.channelsMutex.Lock()
	pc := env.d.protectedChannels[testChannelID]
	pc.AccessChecker = checker
	env.d.protectedChannels[testChannelID] = pc
	env.d.channelsMutex.Unlock()
	return env, store, checker
}

func TestJoinAutoApprovesOnlyPassers(t *testing.T) {
	env, store, checker := newJoinEnv(t)
	d, ctx := env.d, context.Background()
	setJoinMode(t, d, JoinModeAuto)

	d.JoinRequestHandler(ctx, joinUpdate(2, "emp"))
	d.JoinRequestHandler(ctx, joinUpdate(3, "stranger"))

	assert.Equal(t, []int64{2}, joinApproved[testChannelID], "the employee is let in")
	assert.Empty(t, joinDeclined[testChannelID], "failures are left alone, not declined")
	require.Contains(t, store.reqs[testChannelID], int64(3))
	assert.Equal(t, UserStatusBad, store.reqs[testChannelID][3].Check)
	assert.Empty(t, env.sentTexts("Join request"), "auto mode doesn't ping managers")

	// A day later the stranger passes (e.g. got hired): the daily recheck
	// approves them.
	checker.set(3, true)
	env.clock.add(12 * time.Hour)
	d.recheckJoinRequests(ctx)
	assert.Equal(t, []int64{2}, joinApproved[testChannelID], "not due yet")
	env.clock.add(13 * time.Hour)
	d.recheckJoinRequests(ctx)
	assert.Equal(t, []int64{2, 3}, joinApproved[testChannelID])
	assert.Empty(t, store.reqs[testChannelID])
	assert.Equal(t, 1, checker.invalidated[3], "the daily recheck bypasses the checker cache")

	actions := env.audit.actions()
	assert.Contains(t, actions, audit.ActionJoinApproved)
	assert.Contains(t, actions, audit.ActionJoinRequested)
	assert.Contains(t, actions, audit.ActionJoinRechecked)
}

func TestJoinManualKeepsForManagers(t *testing.T) {
	env, store, _ := newJoinEnv(t)
	d, ctx := env.d, context.Background()
	setJoinMode(t, d, JoinModeManual)
	require.NoError(t, d.JoinChannel(ctx, testChannelID, testAdminID))

	d.JoinRequestHandler(ctx, joinUpdate(2, "emp"))
	d.JoinRequestHandler(ctx, joinUpdate(3, "stranger"))
	assert.Empty(t, joinApproved[testChannelID], "manual mode approves nothing by itself")
	assert.Equal(t, 2, env.sentTo(testAdminID, "Join request"), "managers are told about each request in private")

	list, err := d.ListJoinRequests(ctx, testChannelID)
	require.NoError(t, err)
	require.Len(t, list, 2)
	byID := map[int64]JoinRequestView{list[0].UserID: list[0], list[1].UserID: list[1]}
	assert.Equal(t, UserStatusGood, byID[2].Check)
	assert.Equal(t, UserStatusBad, byID[3].Check)

	res, err := d.ResolveJoinRequests(ctx, testChannelID, []int64{2}, true, testAdminID)
	require.NoError(t, err)
	assert.True(t, res[0].OK)
	res, err = d.ResolveJoinRequests(ctx, testChannelID, []int64{3, 404}, false, testAdminID)
	require.NoError(t, err)
	assert.True(t, res[0].OK)
	assert.Equal(t, errRequestNotPending, res[1].Error)
	assert.Equal(t, []int64{2}, joinApproved[testChannelID])
	assert.Equal(t, []int64{3}, joinDeclined[testChannelID])
	assert.Empty(t, store.reqs[testChannelID])

	var approved audit.Event
	for _, e := range env.audit.events {
		if e.Action == audit.ActionJoinApproved {
			approved = e
		}
	}
	assert.Equal(t, testAdminID, approved.ActorID)
	assert.Equal(t, false, approved.Details["auto"])
}

func TestJoinWithdrawnRequestIsDropped(t *testing.T) {
	env, store, _ := newJoinEnv(t)
	d, ctx := env.d, context.Background()
	setJoinMode(t, d, JoinModeManual)
	d.JoinRequestHandler(ctx, joinUpdate(2, "emp"))
	joinMu.Lock()
	joinGone[2] = true
	joinMu.Unlock()

	res, err := d.ResolveJoinRequests(ctx, testChannelID, []int64{2}, true, testAdminID)
	require.NoError(t, err)
	assert.False(t, res[0].OK)
	assert.Contains(t, res[0].Error, "no longer pending")
	assert.Empty(t, store.reqs[testChannelID], "a withdrawn request disappears from the list")
}

func TestJoinOffIgnoresRequestsAndBadModeRejected(t *testing.T) {
	env, store, _ := newJoinEnv(t)
	d, ctx := env.d, context.Background()
	d.JoinRequestHandler(ctx, joinUpdate(2, "emp"))
	assert.Empty(t, store.reqs[testChannelID])
	assert.Empty(t, joinApproved[testChannelID])

	err := d.SetChannelSettings(ctx, testChannelID, ChannelSettings{JoinRequests: "yolo"}, testAdminID)
	assert.ErrorIs(t, err, ErrBadJoinMode)
}

func TestWhitelistTemporaryEntryIsRemoved(t *testing.T) {
	env := newWhitelistDomain(t)
	d, ctx := env.d, context.Background()
	scan, _ := scanUser(t, d, 1)

	_, err := d.AddToWhitelist(ctx, testChannelID, scan.ID, []int64{1}, "", 30*time.Minute, testAdminID)
	assert.ErrorIs(t, err, ErrBadEntryTTL)

	added, err := d.AddToWhitelist(ctx, testChannelID, scan.ID, []int64{1}, "one-week audit", 7*24*time.Hour, testAdminID)
	require.NoError(t, err)
	require.NotNil(t, added[0].DeleteAt)
	assert.Equal(t, env.clock.now().Add(7*24*time.Hour), *added[0].DeleteAt)

	env.clock.add(5 * 24 * time.Hour)
	d.remindWhitelists(ctx)
	assert.Len(t, env.sentTexts("Temporary approval ends soon"), 1)
	assert.Empty(t, env.sentTexts("runs out soon"), "no review reminder when the entry ends first")

	env.clock.add(2*24*time.Hour + time.Minute)
	_, u := scanUser(t, d, 1)
	assert.Equal(t, UserStatusBad, u.Status, "past its end the entry no longer protects")

	d.remindWhitelists(ctx)
	assert.Empty(t, env.wl.entries[testChannelID], "and is removed")
	assert.Len(t, env.sentTexts("Temporary approval ended"), 1)
	last := env.audit.events[0]
	assert.Equal(t, audit.ActionWhitelistRemoved, last.Action)
	assert.Equal(t, "term_ended", last.Details["reason"])
}

type memKnown struct{ chats map[int64]knownchats.Chat }

func (m *memKnown) LoadKnownChats(context.Context) ([]knownchats.Chat, error) {
	out := []knownchats.Chat{}
	for _, c := range m.chats {
		out = append(out, c)
	}
	return out, nil
}
func (m *memKnown) StoreKnownChat(_ context.Context, c knownchats.Chat) error {
	m.chats[c.ID] = c
	return nil
}
func (m *memKnown) DropKnownChat(_ context.Context, id int64) error {
	delete(m.chats, id)
	return nil
}

func TestKnownChatsAndConnect(t *testing.T) {
	env := newWhitelistDomain(t)
	d, ctx := env.d, context.Background()
	known := &memKnown{chats: map[int64]knownchats.Chat{}}
	d.knownChatStorage = known

	membership := func(id int64, status string) *guard.Update {
		return &guard.Update{MyChatMember: &guard.BotMembership{
			Chat:   guard.ChannelInfo{ID: id, Title: "New", Type: guard.ChatTypeChannel},
			From:   guard.User{ID: testAdminID},
			Status: status, CanRestrictMembers: true,
		}}
	}
	d.BotMembershipHandler(ctx, membership(-100900, guard.MemberStatusAdministrator))
	d.BotMembershipHandler(ctx, membership(testChannelID, guard.MemberStatusAdministrator))
	d.BotMembershipHandler(ctx, membership(-100901, guard.MemberStatusAdministrator))
	d.BotMembershipHandler(ctx, membership(-100901, guard.MemberStatusLeft))

	available, err := d.ListAvailableChats(ctx)
	require.NoError(t, err)
	require.Len(t, available, 1, "protected and left chats are not offered")
	assert.Equal(t, int64(-100900), available[0].ID)
	assert.Contains(t, known.chats, int64(-100900))
	assert.NotContains(t, known.chats, int64(-100901))

	require.NoError(t, d.ConnectChat(ctx, -100900, testAdminID))
	pc, ok := d.getProtectedChannel(-100900)
	require.True(t, ok)
	assert.Equal(t, []int64{testAdminID}, pc.CommandChannelIDs, "reports go to the connector's private chat")
	assert.Equal(t, []int64{testAdminID}, pc.Managers)
	assert.Error(t, d.ConnectChat(ctx, -100900, testAdminID), "already protected")
	assert.Error(t, d.ConnectChat(ctx, -100555, testAdminID), "the bot is not there")

	available, _ = d.ListAvailableChats(ctx)
	assert.Empty(t, available)
}

func TestAddFromPrivateChatNeedsEmployeeAndMakesManager(t *testing.T) {
	env := newWhitelistDomain(t)
	d, ctx := env.d, context.Background()
	d.authorizer = denyAll{}
	private := func(userID int64, text string) *guard.Update {
		return &guard.Update{Message: &guard.MessageReceived{
			Message: guard.Message{ChatID: userID, ChatType: guard.ChatTypePrivate, Text: text},
			User:    guard.User{ID: userID},
		}}
	}
	called := 0
	handler := d.requireAuthOrPrivateEmployee(func(context.Context, *guard.Update) { called++ })

	handler(ctx, private(2, "/add")) // idChecker: even IDs pass
	handler(ctx, private(3, "/add"))
	assert.Equal(t, 1, called, "only employees may /add in private")

	group := private(2, "/add")
	group.Message.ChatType = guard.ChatTypeSupergroup
	handler(ctx, group)
	assert.Equal(t, 1, called, "in groups the regular authorizer decides")

	assert.True(t, isStartAdd(private(2, "/start add")))
	assert.False(t, isStartAdd(private(2, "/start")))
}

type denyAll struct{}

func (denyAll) Authorize(context.Context, *guard.Update) (bool, error) { return false, nil }
