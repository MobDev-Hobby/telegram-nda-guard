package webapi

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MobDev-Hobby/telegram-nda-guard/controllers/scanner"
	"github.com/MobDev-Hobby/telegram-nda-guard/processors"
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

	uid, err := verifyWebAppInitData(testBotToken, initDataFor(t, 7, now), time.Hour, now)
	require.NoError(t, err)
	assert.Equal(t, int64(7), uid)

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
	channels map[int64]scanner.ChannelView
	settings map[int64]scanner.ChannelSettings
	kicked   []int64
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
func (f *fakeMiniApp) SetChannelSettings(_ context.Context, id int64, s scanner.ChannelSettings) error {
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

// fakeChannelAuth: user 42 is privileged, user 5 administers channel -100.
type fakeChannelAuth struct{}

func (fakeChannelAuth) AuthorizeChannel(_ context.Context, callerID, channelID int64) (bool, error) {
	return callerID == 42 || (callerID == 5 && channelID == -100), nil
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
	} {
		rec := miniAppRequest(s, 5, c.method, c.path, c.body)
		assert.Equal(t, http.StatusForbidden, rec.Code, c.path)
	}
	assert.Empty(t, app.kicked)
	assert.Empty(t, app.settings)
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
