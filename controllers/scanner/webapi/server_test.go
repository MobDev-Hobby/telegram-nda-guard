package webapi

import (
	"context"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	guard "github.com/MobDev-Hobby/telegram-nda-guard"
	"github.com/MobDev-Hobby/telegram-nda-guard/controllers/scanner"
)

// fakeService is an in-memory ManagementService for handler tests.
type fakeService struct {
	channels map[int64]scanner.ChannelView
	allowed  bool
}

func newFakeService() *fakeService {
	return &fakeService{
		channels: map[int64]scanner.ChannelView{
			123: {ID: 123, Title: "Alpha", AutoScan: true, BotOnChannel: true},
		},
		allowed: true,
	}
}

func (f *fakeService) ListChannels(_ context.Context, _ int64) ([]scanner.ChannelView, error) {
	out := make([]scanner.ChannelView, 0, len(f.channels))
	for _, c := range f.channels {
		out = append(out, c)
	}
	return out, nil
}
func (f *fakeService) GetChannel(_ context.Context, id int64) (scanner.ChannelView, error) {
	if c, ok := f.channels[id]; ok {
		return c, nil
	}
	return scanner.ChannelView{}, errNotFound
}
func (f *fakeService) AddChannel(_ context.Context, id, chat int64, as, ac, al bool) error {
	f.channels[id] = scanner.ChannelView{ID: id, CommandChats: []int64{chat}, AutoScan: as, AutoClean: ac, AllowClean: al}
	return nil
}
func (f *fakeService) RemoveChannel(_ context.Context, id, chat int64) error {
	delete(f.channels, id)
	return nil
}
func (f *fakeService) SetChannelFlags(_ context.Context, id int64, as, ac, al bool) error {
	if c, ok := f.channels[id]; ok {
		c.AutoScan, c.AutoClean, c.AllowClean = as, ac, al
		f.channels[id] = c
		return nil
	}
	return errNotFound
}
func (f *fakeService) ListChannelUsers(_ context.Context, id int64) (scanner.UsersView, error) {
	return scanner.UsersView{ChannelID: id, Title: "Alpha", Good: []guard.User{{ID: 1}}}, nil
}
func (f *fakeService) TriggerScan(_ context.Context, _, _ int64) error  { return nil }
func (f *fakeService) TriggerClean(_ context.Context, _, _ int64) error { return nil }
func (f *fakeService) GetStatus(_ context.Context) (scanner.StatusView, error) {
	return scanner.StatusView{BotUsername: "testbot", Channels: []scanner.ChannelView{{ID: 123}}}, nil
}
func (f *fakeService) RefreshRights(_ context.Context) error { return nil }

// fakeAuth is a WebAuthenticator that allows a single configured user ID.
type fakeAuth struct{ allowedID int64 }

func (a fakeAuth) AuthenticateAndAuthorize(_ context.Context, callerID, _ int64) (bool, error) {
	return callerID == a.allowedID, nil
}

type notFoundErr string

func (e notFoundErr) Error() string { return string(e) }

var errNotFound = notFoundErr("not found")

func newTestServer(t *testing.T, allowedID int64) (*Server, *fakeService) {
	t.Helper()
	svc := newFakeService()
	auth := fakeAuth{allowedID: allowedID}
	s, err := New(svc, auth, "1234567890:TESTbottoken_for_verification", []byte("01234567890123456789012345678901"))
	require.NoError(t, err)
	return s, svc
}

// doWithSession issues req after establishing a valid session cookie for uid.
func doWithSession(t *testing.T, s *Server, uid int64, method, target string, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	// Establish session by issuing one directly.
	s.issueSession(rec, uid)
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.AddCookie(rec.Result().Cookies()[0])
	rec2 := httptest.NewRecorder()
	s.mux.ServeHTTP(rec2, req)
	return rec2
}

func TestUnauthenticatedRequestRejected(t *testing.T) {
	s, _ := newTestServer(t, 42)
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/status", nil))
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestForbiddenUserRejected(t *testing.T) {
	s, _ := newTestServer(t, 42)
	rec := doWithSession(t, s, 999, http.MethodGet, "/api/status", "") // 999 not allowed
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestGetStatusAuthorized(t *testing.T) {
	s, _ := newTestServer(t, 42)
	rec := doWithSession(t, s, 42, http.MethodGet, "/api/status", "")
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "testbot")
}

func TestListChannels(t *testing.T) {
	s, _ := newTestServer(t, 42)
	rec := doWithSession(t, s, 42, http.MethodGet, "/api/channels", "")
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Alpha")
}

func TestGetChannelByID(t *testing.T) {
	s, _ := newTestServer(t, 42)
	rec := doWithSession(t, s, 42, http.MethodGet, "/api/channels/123", "")
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "123")
}

func TestSetChannelFlags(t *testing.T) {
	s, svc := newTestServer(t, 42)
	rec := doWithSession(t, s, 42, http.MethodPatch, "/api/channels/123/flags", `{"autoScan":false,"autoClean":true,"allowClean":true}`)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.False(t, svc.channels[123].AutoScan)
	assert.True(t, svc.channels[123].AutoClean)
}

func TestListChannelUsers(t *testing.T) {
	s, _ := newTestServer(t, 42)
	rec := doWithSession(t, s, 42, http.MethodGet, "/api/channels/123/users", "")
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"good"`)
}

func TestTriggerScanRequiresChat(t *testing.T) {
	s, _ := newTestServer(t, 42)
	rec := doWithSession(t, s, 42, http.MethodPost, "/api/channels/123/scan", "")
	assert.Equal(t, http.StatusBadRequest, rec.Code) // missing ?chat=
}

func TestTriggerScanWithChat(t *testing.T) {
	s, _ := newTestServer(t, 42)
	rec := doWithSession(t, s, 42, http.MethodPost, "/api/channels/123/scan?chat=555", "")
	assert.Equal(t, http.StatusAccepted, rec.Code)
}

func TestAddChannel(t *testing.T) {
	s, _ := newTestServer(t, 42)
	rec := doWithSession(t, s, 42, http.MethodPost, "/api/channels", `{"id":777,"commandChat":555,"autoScan":true}`)
	assert.Equal(t, http.StatusCreated, rec.Code)
}

func TestParsePathID(t *testing.T) {
	cases := []struct {
		path, prefix string
		wantID       int64
		wantSub      string
		wantOK       bool
	}{
		{"/api/channels/123", "/api/channels/", 123, "", true},
		{"/api/channels/123/users", "/api/channels/", 123, "users", true},
		{"/api/channels/abc", "/api/channels/", 0, "", false},
	}
	for _, c := range cases {
		id, sub, ok := parsePathID(c.path, c.prefix)
		assert.Equal(t, c.wantOK, ok, c.path)
		assert.Equal(t, c.wantID, id, c.path)
		assert.Equal(t, c.wantSub, sub, c.path)
	}
}

func TestSessionRoundTrip(t *testing.T) {
	s, _ := newTestServer(t, 42)
	rec := httptest.NewRecorder()
	s.issueSession(rec, 42)
	cookies := rec.Result().Cookies()
	require.Len(t, cookies, 1)
	// Validate the cookie decodes back to 42.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(cookies[0])
	uid, ok := s.validateSession(req)
	assert.True(t, ok)
	assert.Equal(t, int64(42), uid)
}

func TestTamperedSessionRejected(t *testing.T) {
	s, _ := newTestServer(t, 42)
	rec := httptest.NewRecorder()
	s.issueSession(rec, 42)
	cookie := rec.Result().Cookies()[0]
	// Flip a character in the hex value to break the MAC.
	raw, _ := hex.DecodeString(cookie.Value)
	raw[0] ^= 0xFF
	cookie.Value = hex.EncodeToString(raw)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(cookie)
	_, ok := s.validateSession(req)
	assert.False(t, ok, "tampered session must be rejected")
}

func TestNewRejectsBadConfig(t *testing.T) {
	_, err := New(nil, fakeAuth{1}, "tok", []byte("01234567890123456789012345678901"))
	assert.Error(t, err)
	_, err = New(newFakeService(), nil, "tok", []byte("01234567890123456789012345678901"))
	assert.Error(t, err)
	_, err = New(newFakeService(), fakeAuth{1}, "tok", []byte("short"))
	assert.Error(t, err)
	_, err = New(newFakeService(), fakeAuth{1}, "", []byte("01234567890123456789012345678901"))
	assert.Error(t, err)
}
