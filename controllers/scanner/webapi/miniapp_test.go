package webapi

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	guard "github.com/MobDev-Hobby/telegram-nda-guard"
	"github.com/MobDev-Hobby/telegram-nda-guard/controllers/scanner"
	"github.com/MobDev-Hobby/telegram-nda-guard/processors"
	"github.com/MobDev-Hobby/telegram-nda-guard/storage/audit"
	"github.com/MobDev-Hobby/telegram-nda-guard/storage/joinrequests"
	"github.com/MobDev-Hobby/telegram-nda-guard/storage/knownchats"
	"github.com/MobDev-Hobby/telegram-nda-guard/storage/whitelist"
)

const testBotToken = "1234567890:TESTbottoken_for_verification"

// signInitData builds initData the way Telegram does.
func signInitData(t *testing.T, token string, fields map[string]string) string {
	t.Helper()
	lines := make([]string, 0, len(fields))
	for k, v := range fields {
		lines = append(lines, k+"="+v)
	}
	sort.Strings(lines)
	secret := hmac.New(sha256.New, []byte("WebAppData"))
	secret.Write([]byte(token))
	mac := hmac.New(sha256.New, secret.Sum(nil))
	mac.Write([]byte(strings.Join(lines, "\n")))

	values := url.Values{}
	for k, v := range fields {
		values.Set(k, v)
	}
	values.Set("hash", hex.EncodeToString(mac.Sum(nil)))
	return values.Encode()
}

func initDataFor(t *testing.T, userID int64, at time.Time) string {
	return signInitData(t, testBotToken, map[string]string{
		"auth_date": strconv.FormatInt(at.Unix(), 10),
		"query_id":  "AAH",
		"user":      `{"id":` + strconv.FormatInt(userID, 10) + `,"first_name":"Ann"}`,
	})
}

func TestVerifyWebAppInitData(t *testing.T) {
	now := time.Now()

	user, err := verifyWebAppInitData(testBotToken, initDataFor(t, 7, now), time.Hour, now)
	require.NoError(t, err)
	assert.Equal(t, int64(7), user.ID)
	assert.Equal(t, "Ann", user.FirstName)

	// Signed with another bot's token.
	forged := signInitData(t, "999:other", map[string]string{
		"auth_date": strconv.FormatInt(now.Unix(), 10),
		"user":      `{"id":7}`,
	})
	_, err = verifyWebAppInitData(testBotToken, forged, time.Hour, now)
	assert.Error(t, err)

	// Tampered user after signing.
	tampered := strings.Replace(initDataFor(t, 7, now), "%22id%22%3A7", "%22id%22%3A8", 1)
	_, err = verifyWebAppInitData(testBotToken, tampered, time.Hour, now)
	assert.Error(t, err)

	// Too old.
	_, err = verifyWebAppInitData(testBotToken, initDataFor(t, 7, now.Add(-2*time.Hour)), time.Hour, now)
	assert.Error(t, err)
}

type fakeMiniApp struct {
	channels   map[int64]scanner.ChannelView
	settings   map[int64]scanner.ChannelSettings
	kicked     []int64
	whitelist  []scanner.WhitelistEntryView
	managers   map[int64][]int64
	rechecked  []int64
	auditLimit int
	lastTTL    time.Duration
	connected  []int64
	resolved   map[bool][]int64
}

func (f *fakeMiniApp) ListAvailableChats(context.Context) ([]knownchats.Chat, error) {
	return []knownchats.Chat{{ID: -100, Title: "Mine"}, {ID: -300, Title: "Not mine"}}, nil
}
func (f *fakeMiniApp) ConnectChat(_ context.Context, chatID, _ int64) error {
	f.connected = append(f.connected, chatID)
	return nil
}
func (f *fakeMiniApp) BotUsername() string { return "guardbot" }
func (f *fakeMiniApp) ListJoinRequests(context.Context, int64) ([]scanner.JoinRequestView, error) {
	return []scanner.JoinRequestView{{Request: joinrequests.Request{UserID: 8, Check: "bad"}}}, nil
}
func (f *fakeMiniApp) ResolveJoinRequests(_ context.Context, _ int64, ids []int64, approve bool, _ int64) ([]scanner.JoinResult, error) {
	if f.resolved == nil {
		f.resolved = map[bool][]int64{}
	}
	f.resolved[approve] = append(f.resolved[approve], ids...)
	return []scanner.JoinResult{{UserID: ids[0], OK: true}}, nil
}
func (f *fakeMiniApp) RecheckJoinRequest(_ context.Context, _, userID, _ int64) (scanner.JoinRequestView, error) {
	return scanner.JoinRequestView{Request: joinrequests.Request{UserID: userID, Check: "good"}}, nil
}

func (f *fakeMiniApp) ListChannels(context.Context, int64) ([]scanner.ChannelView, error) {
	out := make([]scanner.ChannelView, 0, len(f.channels))
	for _, c := range f.channels {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}
func (f *fakeMiniApp) GetChannel(_ context.Context, id int64) (scanner.ChannelView, error) {
	return f.channels[id], nil
}
func (f *fakeMiniApp) SetChannelSettings(_ context.Context, id int64, s scanner.ChannelSettings, _ int64) error {
	f.settings[id] = s
	return nil
}
func (f *fakeMiniApp) StartChannelScan(_ context.Context, id, _ int64) (scanner.ScanView, error) {
	return scanner.ScanView{ID: "s1", ChannelID: id, State: scanner.ScanStateRunning}, nil
}
func (f *fakeMiniApp) GetChannelScan(_ context.Context, id int64, scanID string) (scanner.ScanView, error) {
	if scanID != "s1" {
		return scanner.ScanView{}, scanner.ErrScanNotFound
	}
	return scanner.ScanView{ID: scanID, ChannelID: id, State: scanner.ScanStateDone}, nil
}
func (f *fakeMiniApp) KickScannedUsers(_ context.Context, _ int64, _ string, ids []int64, _ int64) ([]processors.KickResult, error) {
	f.kicked = append(f.kicked, ids...)
	return []processors.KickResult{{UserID: ids[0], OK: true}}, nil
}

func (f *fakeMiniApp) ListWhitelist(_ context.Context, id int64) ([]scanner.WhitelistEntryView, error) {
	return f.whitelist, nil
}
func (f *fakeMiniApp) AddToWhitelist(_ context.Context, _ int64, _ string, ids []int64, note string, ttl time.Duration, callerID int64) ([]scanner.WhitelistEntryView, error) {
	f.lastTTL = ttl
	for _, id := range ids {
		f.whitelist = append(f.whitelist, scanner.WhitelistEntryView{Entry: whitelist.Entry{UserID: id, ApprovedBy: callerID, Note: note}})
	}
	return f.whitelist, nil
}
func (f *fakeMiniApp) RenewWhitelistEntry(_ context.Context, _, userID int64, note string, _ time.Duration, callerID int64) (scanner.WhitelistEntryView, error) {
	for i := range f.whitelist {
		if f.whitelist[i].UserID == userID {
			f.whitelist[i].ApprovedBy = callerID
			if note != "" {
				f.whitelist[i].Note = note
			}
			return f.whitelist[i], nil
		}
	}
	return scanner.WhitelistEntryView{}, scanner.ErrNotWhitelisted
}
func (f *fakeMiniApp) RemoveWhitelistEntry(_ context.Context, _, userID, _ int64) error {
	for i := range f.whitelist {
		if f.whitelist[i].UserID == userID {
			f.whitelist = append(f.whitelist[:i], f.whitelist[i+1:]...)
			return nil
		}
	}
	return scanner.ErrNotWhitelisted
}

func (f *fakeMiniApp) RecheckScannedUser(_ context.Context, _ int64, scanID string, userID, _ int64) (scanner.ScannedUser, error) {
	if scanID != "s1" {
		return scanner.ScannedUser{}, scanner.ErrScanNotFound
	}
	f.rechecked = append(f.rechecked, userID)
	return scanner.ScannedUser{ID: userID, Status: scanner.UserStatusGood, Check: scanner.UserStatusGood}, nil
}
func (f *fakeMiniApp) CanUseMiniApp(_ context.Context, u guard.User) (bool, error) {
	if u.ID == 13 {
		return false, errors.New("ap-search down")
	}
	return u.ID != 9, nil // 9 is not an employee
}
func (f *fakeMiniApp) NoteUserName(int64, string) {}
func (f *fakeMiniApp) IsChannelManager(channelID, userID int64) bool {
	return slices.Contains(f.managers[channelID], userID)
}
func (f *fakeMiniApp) JoinChannel(_ context.Context, channelID, userID int64) error {
	f.managers[channelID] = append(f.managers[channelID], userID)
	return nil
}
func (f *fakeMiniApp) ListAudit(_ context.Context, _ int64, limit int) ([]audit.Event, error) {
	f.auditLimit = limit
	return []audit.Event{{Action: audit.ActionUsersKicked, ActorID: 5}}, nil
}

// fakeChannelAuth: user 42 is privileged; user 5 administers channel -100;
// user 6 administers -100 but has not joined it in the bot.
type fakeChannelAuth struct{}

func (fakeChannelAuth) AuthorizeChannel(_ context.Context, callerID, channelID int64) (bool, error) {
	return callerID == 42 || ((callerID == 5 || callerID == 6) && channelID == -100), nil
}
func (fakeChannelAuth) IsPrivileged(_ context.Context, callerID int64) bool { return callerID == 42 }

func newMiniAppServer(t *testing.T) (*Server, *fakeMiniApp) {
	t.Helper()
	app := &fakeMiniApp{
		channels: map[int64]scanner.ChannelView{
			-100: {ID: -100, Title: "Mine"},
			-200: {ID: -200, Title: "Someone else's"},
		},
		settings: map[int64]scanner.ChannelSettings{},
		managers: map[int64][]int64{-100: {5}},
	}
	s, err := New(newFakeService(), fakeAuth{allowedID: 42}, testBotToken,
		[]byte("01234567890123456789012345678901"), WithMiniApp(app, fakeChannelAuth{}))
	require.NoError(t, err)
	return s, app
}

func miniAppRequest(s *Server, uid int64, method, target, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+s.newSessionToken(uid))
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	return rec
}

func TestMiniAppAuthIssuesToken(t *testing.T) {
	s, _ := newMiniAppServer(t)
	body, _ := json.Marshal(map[string]string{"initData": initDataFor(t, 5, time.Now())})
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/miniapp/auth", strings.NewReader(string(body))))
	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Token      string `json:"token"`
		UserID     int64  `json:"userId"`
		Privileged bool   `json:"privileged"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, int64(5), resp.UserID)
	assert.False(t, resp.Privileged)
	uid, ok := s.validateSessionToken(resp.Token)
	assert.True(t, ok)
	assert.Equal(t, int64(5), uid)
}

func TestMiniAppAuthRejectsForgedInitData(t *testing.T) {
	s, _ := newMiniAppServer(t)
	forged := signInitData(t, "999:other", map[string]string{
		"auth_date": strconv.FormatInt(time.Now().Unix(), 10),
		"user":      `{"id":42}`,
	})
	body, _ := json.Marshal(map[string]string{"initData": forged})
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/miniapp/auth", strings.NewReader(string(body))))
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestMiniAppListsOnlyAdministeredChannels(t *testing.T) {
	s, _ := newMiniAppServer(t)

	rec := miniAppRequest(s, 5, http.MethodGet, "/api/miniapp/channels", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Mine")
	assert.NotContains(t, rec.Body.String(), "Someone else")

	rec = miniAppRequest(s, 42, http.MethodGet, "/api/miniapp/channels", "")
	assert.Contains(t, rec.Body.String(), "Someone else")
}

func TestMiniAppForbidsForeignChannel(t *testing.T) {
	s, app := newMiniAppServer(t)
	for _, c := range []struct{ method, path, body string }{
		{http.MethodGet, "/api/miniapp/channels/-200", ""},
		{http.MethodPut, "/api/miniapp/channels/-200/settings", `{"autoScan":true}`},
		{http.MethodPost, "/api/miniapp/channels/-200/scans", ""},
		{http.MethodPost, "/api/miniapp/channels/-200/kick", `{"scanId":"s1","userIds":[1]}`},
		{http.MethodGet, "/api/miniapp/channels/-200/whitelist", ""},
		{http.MethodPost, "/api/miniapp/channels/-200/whitelist", `{"scanId":"s1","userIds":[1]}`},
		{http.MethodPost, "/api/miniapp/channels/-200/whitelist/1/renew", ""},
		{http.MethodDelete, "/api/miniapp/channels/-200/whitelist/1", ""},
	} {
		rec := miniAppRequest(s, 5, c.method, c.path, c.body)
		assert.Equal(t, http.StatusForbidden, rec.Code, c.path)
	}
	assert.Empty(t, app.kicked)
	assert.Empty(t, app.settings)
	assert.Empty(t, app.whitelist)
}

func TestMiniAppRequiresSession(t *testing.T) {
	s, _ := newMiniAppServer(t)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/miniapp/channels", nil))
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestMiniAppScanAndKickFlow(t *testing.T) {
	s, app := newMiniAppServer(t)

	rec := miniAppRequest(s, 5, http.MethodPost, "/api/miniapp/channels/-100/scans", "")
	require.Equal(t, http.StatusAccepted, rec.Code)
	rec = miniAppRequest(s, 5, http.MethodGet, "/api/miniapp/channels/-100/scans/s1", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"state":"done"`)
	rec = miniAppRequest(s, 5, http.MethodGet, "/api/miniapp/channels/-100/scans/nope", "")
	assert.Equal(t, http.StatusNotFound, rec.Code)

	rec = miniAppRequest(s, 5, http.MethodPost, "/api/miniapp/channels/-100/kick", `{"scanId":"s1","userIds":[11]}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, []int64{11}, app.kicked)

	rec = miniAppRequest(s, 5, http.MethodPost, "/api/miniapp/channels/-100/kick", `{"scanId":"s1","userIds":[]}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestMiniAppSaveSettings(t *testing.T) {
	s, app := newMiniAppServer(t)
	rec := miniAppRequest(s, 5, http.MethodPut, "/api/miniapp/channels/-100/settings",
		`{"autoScan":true,"autoClean":false,"allowClean":true,"keepBanned":true,"cleanMessages":false,"cleanUnknown":true}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, scanner.ChannelSettings{
		AutoScan: true, AllowClean: true, KeepBanned: true, CleanUnknown: true,
	}, app.settings[-100])
}

func TestMiniAppPageServed(t *testing.T) {
	s, _ := newMiniAppServer(t)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/miniapp/", nil))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
}

func TestMiniAppDisabledByDefault(t *testing.T) {
	s, _ := newTestServer(t, 42)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/miniapp/auth", nil))
	assert.NotEqual(t, http.StatusOK, rec.Code)
}

func TestMiniAppWhitelistFlow(t *testing.T) {
	s, app := newMiniAppServer(t)

	rec := miniAppRequest(s, 5, http.MethodPost, "/api/miniapp/channels/-100/whitelist", `{"scanId":"s1","userIds":[11],"note":"agency, contract #12"}`)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Len(t, app.whitelist, 1)
	assert.Equal(t, "agency, contract #12", app.whitelist[0].Note)
	assert.Equal(t, int64(5), app.whitelist[0].ApprovedBy, "approver is the caller, not a body field")

	rec = miniAppRequest(s, 5, http.MethodGet, "/api/miniapp/channels/-100/whitelist", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"userId":11`)
	assert.Contains(t, rec.Body.String(), `"expired":false`)

	rec = miniAppRequest(s, 42, http.MethodPost, "/api/miniapp/channels/-100/whitelist/11/renew", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, int64(42), app.whitelist[0].ApprovedBy)

	rec = miniAppRequest(s, 5, http.MethodPost, "/api/miniapp/channels/-100/whitelist/99/renew", "")
	assert.Equal(t, http.StatusNotFound, rec.Code)
	rec = miniAppRequest(s, 5, http.MethodPost, "/api/miniapp/channels/-100/whitelist/abc/renew", "")
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	rec = miniAppRequest(s, 5, http.MethodDelete, "/api/miniapp/channels/-100/whitelist/11", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, app.whitelist)
}

func postAuth(t *testing.T, s *Server, initData string) *httptest.ResponseRecorder {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"initData": initData})
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/miniapp/auth", strings.NewReader(string(body))))
	return rec
}

func TestMiniAppAuthOnlyForEmployees(t *testing.T) {
	s, _ := newMiniAppServer(t)

	rec := postAuth(t, s, initDataFor(t, 9, time.Now()))
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Body.String(), `"code":"not_employee"`)

	rec = postAuth(t, s, initDataFor(t, 13, time.Now()))
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code, "a failed check is not a pass")
}

func TestMiniAppSessionIsShort(t *testing.T) {
	s, _ := newMiniAppServer(t)
	rec := postAuth(t, s, initDataFor(t, 5, time.Now()))
	require.Equal(t, http.StatusOK, rec.Code)
	var resp struct {
		Token string `json:"token"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	raw, err := hex.DecodeString(resp.Token)
	require.NoError(t, err)
	expiry := time.Unix(int64(binary.BigEndian.Uint64(raw[8:16])), 0)
	assert.WithinDuration(t, time.Now().Add(time.Hour), expiry, time.Minute)
}

func TestMiniAppJoinFlow(t *testing.T) {
	s, app := newMiniAppServer(t)

	// User 6 administers -100 in Telegram but hasn't joined it in the bot.
	rec := miniAppRequest(s, 6, http.MethodGet, "/api/miniapp/channels", "")
	require.Equal(t, http.StatusOK, rec.Code)
	var list []MiniAppChannel
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &list))
	require.Len(t, list, 1)
	assert.False(t, list[0].Joined)

	rec = miniAppRequest(s, 6, http.MethodPost, "/api/miniapp/channels/-100/scans", "")
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Body.String(), `"code":"not_joined"`)

	rec = miniAppRequest(s, 6, http.MethodPost, "/api/miniapp/channels/-100/join", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, app.managers[-100], int64(6))

	rec = miniAppRequest(s, 6, http.MethodPost, "/api/miniapp/channels/-100/scans", "")
	assert.Equal(t, http.StatusAccepted, rec.Code)

	// Joining needs Telegram admin rights on the channel.
	rec = miniAppRequest(s, 6, http.MethodPost, "/api/miniapp/channels/-200/join", "")
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.NotContains(t, app.managers[-200], int64(6))
}

func TestMiniAppRecheckAndAudit(t *testing.T) {
	s, app := newMiniAppServer(t)

	rec := miniAppRequest(s, 5, http.MethodPost, "/api/miniapp/channels/-100/scans/s1/users/77/recheck", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, []int64{77}, app.rechecked)
	assert.Contains(t, rec.Body.String(), `"check":"good"`)

	rec = miniAppRequest(s, 5, http.MethodPost, "/api/miniapp/channels/-100/scans/old/users/77/recheck", "")
	assert.Equal(t, http.StatusNotFound, rec.Code)
	rec = miniAppRequest(s, 5, http.MethodPost, "/api/miniapp/channels/-100/scans/s1/users/x/recheck", "")
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	rec = miniAppRequest(s, 5, http.MethodGet, "/api/miniapp/channels/-100/audit?limit=50", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), audit.ActionUsersKicked)
	assert.Equal(t, 50, app.auditLimit)

	rec = miniAppRequest(s, 5, http.MethodGet, "/api/miniapp/channels/-200/audit", "")
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestMiniAppConnectAvailableChat(t *testing.T) {
	s, app := newMiniAppServer(t)

	rec := miniAppRequest(s, 5, http.MethodGet, "/api/miniapp/available", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Mine")
	assert.NotContains(t, rec.Body.String(), "Not mine", "only chats the caller administers")

	rec = miniAppRequest(s, 5, http.MethodPost, "/api/miniapp/available/-300/connect", "")
	assert.Equal(t, http.StatusForbidden, rec.Code)
	rec = miniAppRequest(s, 5, http.MethodPost, "/api/miniapp/available/-100/connect", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, []int64{-100}, app.connected)
}

func TestMiniAppJoinRequests(t *testing.T) {
	s, app := newMiniAppServer(t)

	rec := miniAppRequest(s, 5, http.MethodGet, "/api/miniapp/channels/-100/joins", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"userId":8`)

	rec = miniAppRequest(s, 5, http.MethodPost, "/api/miniapp/channels/-100/joins/approve", `{"userIds":[8]}`)
	require.Equal(t, http.StatusOK, rec.Code)
	rec = miniAppRequest(s, 5, http.MethodPost, "/api/miniapp/channels/-100/joins/decline", `{"userIds":[9]}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, []int64{8}, app.resolved[true])
	assert.Equal(t, []int64{9}, app.resolved[false])

	rec = miniAppRequest(s, 5, http.MethodPost, "/api/miniapp/channels/-100/joins/8/recheck", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"check":"good"`)

	rec = miniAppRequest(s, 5, http.MethodPost, "/api/miniapp/channels/-200/joins/approve", `{"userIds":[8]}`)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestMiniAppWhitelistTTLDays(t *testing.T) {
	s, app := newMiniAppServer(t)
	rec := miniAppRequest(s, 5, http.MethodPost, "/api/miniapp/channels/-100/whitelist", `{"scanId":"s1","userIds":[11],"ttlDays":7}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, 7*24*time.Hour, app.lastTTL)
}

func TestMiniAppAuthReturnsBotUsername(t *testing.T) {
	s, _ := newMiniAppServer(t)
	rec := postAuth(t, s, initDataFor(t, 5, time.Now()))
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"botUsername":"guardbot"`)
}
