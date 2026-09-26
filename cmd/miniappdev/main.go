// Command miniappdev serves the real Mini App (page + API) against an
// in-memory fake service, for UI work without Telegram, Redis or a bot token.
//
//	go run ./cmd/miniappdev   # then open http://127.0.0.1:8797/start
//
// /start signs Telegram initData for user 5 (an employee) and redirects to
// the app; /start?uid=9 signs in as a non-employee. PORT overrides the port.
package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	guard "github.com/MobDev-Hobby/telegram-nda-guard"
	"github.com/MobDev-Hobby/telegram-nda-guard/controllers/scanner"
	"github.com/MobDev-Hobby/telegram-nda-guard/controllers/scanner/webapi"
	"github.com/MobDev-Hobby/telegram-nda-guard/processors"
	"github.com/MobDev-Hobby/telegram-nda-guard/storage/audit"
	"github.com/MobDev-Hobby/telegram-nda-guard/storage/joinrequests"
	"github.com/MobDev-Hobby/telegram-nda-guard/storage/knownchats"
	"github.com/MobDev-Hobby/telegram-nda-guard/storage/whitelist"
)

const token = "123:devtoken"

type svc struct {
	mu       sync.Mutex
	channels map[int64]scanner.ChannelView
	managers map[int64]bool
	scan     *scanner.ScanView
	started  time.Time
	wl       map[int64]scanner.WhitelistEntryView
	joins    []scanner.JoinRequestView
	events   []audit.Event
	avail    []knownchats.Chat
}

func now() time.Time { return time.Now() }

func (s *svc) log(action string, users []audit.User, details map[string]any) {
	s.events = append([]audit.Event{{At: now(), ActorID: 5, ActorName: "Anna Admin", Action: action, Users: users, Details: details}}, s.events...)
}

func (s *svc) ListChannels(context.Context, int64) ([]scanner.ChannelView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []scanner.ChannelView{}
	for _, c := range s.channels {
		out = append(out, c)
	}
	return out, nil
}
func (s *svc) GetChannel(_ context.Context, id int64) (scanner.ChannelView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.channels[id], nil
}
func (s *svc) SetChannelSettings(_ context.Context, id int64, st scanner.ChannelSettings, _ int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	c := s.channels[id]
	c.AutoScan, c.AutoClean, c.AllowClean = st.AutoScan, st.AutoClean, st.AllowClean
	c.KeepBanned, c.CleanMessages, c.CleanUnknown, c.CustomCleanOptions = st.KeepBanned, st.CleanMessages, st.CleanUnknown, true
	c.JoinRequests = st.JoinRequests
	s.channels[id] = c
	s.log(audit.ActionSettingsChanged, nil, nil)
	return nil
}
func (s *svc) StartChannelScan(_ context.Context, id, _ int64) (scanner.ScanView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.started = now()
	s.scan = &scanner.ScanView{ID: "dev1", ChannelID: id, Title: s.channels[id].Title, State: scanner.ScanStateRunning}
	return *s.scan, nil
}
func (s *svc) GetChannelScan(context.Context, int64, string) (scanner.ScanView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.scan == nil {
		return scanner.ScanView{}, scanner.ErrScanNotFound
	}
	el := time.Since(s.started)
	v := *s.scan
	if v.State == scanner.ScanStateRunning {
		if el > 800*time.Millisecond {
			v.Stats = guard.ScanStats{Fetched: 9, Total: 200}
			v.Checked = min(int(el/(300*time.Millisecond)), 9)
		}
		if el > 3000*time.Millisecond {
			until := now().Add(12 * 24 * time.Hour)
			past := now().Add(-3 * 24 * time.Hour)
			del := now().Add(5 * 24 * time.Hour)
			v.State, v.Partial, v.Checked = scanner.ScanStateDone, true, 9
			v.Users = []scanner.ScannedUser{
				{ID: 1, FirstName: "Иван", LastName: "Внешний", Username: "ivan_ext", Status: "bad"},
				{ID: 2, FirstName: "<script>alert(1)</script>", Status: "bad"},
				{ID: 3, FirstName: "Lena", Username: "lena", Status: "bad"},
				{ID: 4, FirstName: "Петр", Status: "unknown"},
				{ID: 10, FirstName: "Подрядчик", Username: "contractor", Status: "whitelisted", Protected: true, Note: "whitelisted", Whitelist: &scanner.ScannedWhitelist{Until: past, Expired: true, Note: "Агентство, договор №12"}},
				{ID: 11, FirstName: "Стажёр", Status: "whitelisted", Protected: true, Note: "whitelisted", Whitelist: &scanner.ScannedWhitelist{Until: until, DeleteAt: &del}},
				{ID: 5, FirstName: "Мария", LastName: "Сотрудник", Username: "maria", Status: "good"},
				{ID: 6, FirstName: "Админ", Username: "boss", Status: "good", Protected: true, Note: "administrator"},
				{ID: 7, FirstName: "NDA Guard", Username: "guardbot", Status: "good", Protected: true, Note: "this bot"},
			}
			*s.scan = v
		}
	}
	return v, nil
}
func (s *svc) KickScannedUsers(_ context.Context, _ int64, _ string, ids []int64, _ int64) ([]processors.KickResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.scan == nil {
		return nil, scanner.ErrScanNotFound
	}
	out := []processors.KickResult{}
	for _, uid := range ids {
		if uid == 3 {
			out = append(out, processors.KickResult{UserID: uid, Error: "USER_NOT_PARTICIPANT"})
			continue
		}
		out = append(out, processors.KickResult{UserID: uid, OK: true})
		for i := range s.scan.Users {
			if s.scan.Users[i].ID == uid {
				s.scan.Users[i].Status = "kicked"
			}
		}
	}
	s.log(audit.ActionUsersKicked, []audit.User{{ID: 1, Name: "Иван Внешний"}}, nil)
	return out, nil
}
func (s *svc) RecheckScannedUser(_ context.Context, _ int64, _ string, userID, _ int64) (scanner.ScannedUser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.scan == nil {
		return scanner.ScannedUser{}, scanner.ErrScanNotFound
	}
	t := now()
	for i := range s.scan.Users {
		u := &s.scan.Users[i]
		if u.ID != userID {
			continue
		}
		u.Check, u.CheckedAt = "good", &t
		if u.ID == 10 {
			u.Check = "bad"
		}
		if u.Status != "whitelisted" {
			u.Status = u.Check
		}
		s.log(audit.ActionUserRechecked, []audit.User{{ID: u.ID, Name: u.FirstName}}, map[string]any{"result": u.Check})
		return *u, nil
	}
	return scanner.ScannedUser{}, errors.New("not found")
}
func (s *svc) ListWhitelist(context.Context, int64) ([]scanner.WhitelistEntryView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []scanner.WhitelistEntryView{}
	for _, e := range s.wl {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ExpiresAt.Before(out[j].ExpiresAt) })
	return out, nil
}
func (s *svc) AddToWhitelist(_ context.Context, _ int64, _ string, ids []int64, note string, ttl time.Duration, caller int64) ([]scanner.WhitelistEntryView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.scan == nil {
		return nil, scanner.ErrScanNotFound
	}
	if len(ids) == 0 {
		return nil, errors.New("no users selected")
	}
	out := []scanner.WhitelistEntryView{}
	for _, id := range ids {
		for i := range s.scan.Users {
			u := &s.scan.Users[i]
			if u.ID != id {
				continue
			}
			e := whitelist.Entry{UserID: id, FirstName: u.FirstName, Username: u.Username, ApprovedBy: caller, ApprovedAt: now(), ExpiresAt: now().Add(30 * 24 * time.Hour), Note: note}
			if ttl > 0 {
				d := now().Add(ttl)
				e.DeleteAt = &d
			}
			s.wl[id] = scanner.WhitelistEntryView{Entry: e}
			u.Status, u.Protected, u.Note = "whitelisted", true, "whitelisted"
			u.Whitelist = &scanner.ScannedWhitelist{Until: e.ExpiresAt, Note: note, DeleteAt: e.DeleteAt}
			out = append(out, s.wl[id])
		}
	}
	s.log(audit.ActionWhitelistAdded, []audit.User{{ID: ids[0], Name: "Иван"}}, nil)
	return out, nil
}
func (s *svc) RenewWhitelistEntry(_ context.Context, _, id int64, note string, _ time.Duration, caller int64) (scanner.WhitelistEntryView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e := s.wl[id]
	e.ExpiresAt, e.ApprovedBy, e.Expired = now().Add(30*24*time.Hour), caller, false
	if note != "" {
		e.Note = note
	}
	s.wl[id] = e
	s.log(audit.ActionWhitelistRenewed, []audit.User{{ID: id, Name: e.FirstName}}, nil)
	return e, nil
}
func (s *svc) RemoveWhitelistEntry(_ context.Context, _, id, _ int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.wl, id)
	return nil
}
func (s *svc) CanUseMiniApp(_ context.Context, u guard.User) (bool, error) { return u.ID != 9, nil }
func (s *svc) NoteUserName(int64, string)                                  {}
func (s *svc) IsChannelManager(id, _ int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.managers[id]
}
func (s *svc) JoinChannel(_ context.Context, id, _ int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.managers[id] = true
	return nil
}
func (s *svc) ListAudit(context.Context, int64, int) ([]audit.Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.events, nil
}
func (s *svc) ListAvailableChats(context.Context) ([]knownchats.Chat, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.avail, nil
}
func (s *svc) ConnectChat(_ context.Context, id, _ int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, c := range s.avail {
		if c.ID == id {
			s.avail = append(s.avail[:i], s.avail[i+1:]...)
			s.channels[id] = scanner.ChannelView{ID: id, Title: c.Title, ChatType: c.Type, BotOnChannel: true, BotCanClean: true, AllowClean: true, WhitelistEnabled: true, WhitelistTTLDays: 30, JoinRequestsEnabled: true, JoinRequests: "off", Health: scanner.ChannelHealth{Status: "yellow", Reason: "never_checked"}}
			s.managers[id] = true
		}
	}
	return nil
}
func (s *svc) BotUsername() string { return "nda_guard_bot" }
func (s *svc) ListJoinRequests(context.Context, int64) ([]scanner.JoinRequestView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.joins, nil
}
func (s *svc) ResolveJoinRequests(_ context.Context, _ int64, ids []int64, approve bool, _ int64) ([]scanner.JoinResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []scanner.JoinResult{}
	keep := s.joins[:0]
	for _, r := range s.joins {
		hit := false
		for _, id := range ids {
			hit = hit || id == r.UserID
		}
		if hit {
			out = append(out, scanner.JoinResult{UserID: r.UserID, OK: true})
		} else {
			keep = append(keep, r)
		}
	}
	s.joins = keep
	if len(ids) == 0 {
		return out, nil
	}
	act := audit.ActionJoinDeclined
	if approve {
		act = audit.ActionJoinApproved
	}
	s.log(act, []audit.User{{ID: ids[0], Name: "Заявитель"}}, map[string]any{"auto": false})
	return out, nil
}
func (s *svc) RecheckJoinRequest(_ context.Context, _, id, _ int64) (scanner.JoinRequestView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.joins {
		if s.joins[i].UserID == id {
			s.joins[i].Check, s.joins[i].CheckedAt = "good", now()
			return s.joins[i], nil
		}
	}
	return scanner.JoinRequestView{}, scanner.ErrJoinRequestNotFound
}

// ManagementService stubs.
func (s *svc) AddChannel(context.Context, int64, int64, bool, bool, bool) error { return nil }
func (s *svc) RemoveChannel(context.Context, int64, int64) error                { return nil }
func (s *svc) SetChannelFlags(context.Context, int64, bool, bool, bool) error   { return nil }
func (s *svc) ListChannelUsers(context.Context, int64) (scanner.UsersView, error) {
	return scanner.UsersView{}, nil
}
func (s *svc) TriggerScan(context.Context, int64, int64) error  { return nil }
func (s *svc) TriggerClean(context.Context, int64, int64) error { return nil }
func (s *svc) GetStatus(context.Context) (scanner.StatusView, error) {
	return scanner.StatusView{}, nil
}
func (s *svc) RefreshRights(context.Context) error { return nil }

type auth struct{}

func (auth) AuthorizeChannel(context.Context, int64, int64) (bool, error)         { return true, nil }
func (auth) IsPrivileged(context.Context, int64) bool                             { return false }
func (auth) AuthenticateAndAuthorize(context.Context, int64, int64) (bool, error) { return true, nil }

func initData(uid int64) string {
	f := map[string]string{"auth_date": strconv.FormatInt(now().Unix(), 10), "user": fmt.Sprintf(`{"id":%d,"first_name":"Ann","language_code":"ru"}`, uid)}
	lines := []string{}
	for k, v := range f {
		lines = append(lines, k+"="+v)
	}
	sort.Strings(lines)
	sec := hmac.New(sha256.New, []byte("WebAppData"))
	sec.Write([]byte(token))
	m := hmac.New(sha256.New, sec.Sum(nil))
	m.Write([]byte(strings.Join(lines, "\n")))
	v := url.Values{}
	for k, val := range f {
		v.Set(k, val)
	}
	v.Set("hash", hex.EncodeToString(m.Sum(nil)))
	return v.Encode()
}

func main() {
	day := 24 * time.Hour
	check := func(at time.Time, bad int) *processors.CheckSummary {
		return &processors.CheckSummary{At: at, Bad: bad, Good: 40}
	}
	base := scanner.ChannelView{BotOnChannel: true, BotCanClean: true, BotCanInvite: true, AllowClean: true, WhitelistEnabled: true, WhitelistTTLDays: 30, JoinRequestsEnabled: true, JoinRequests: "off", CleanMessages: true}
	mk := func(id int64, title, typ string, h scanner.ChannelHealth, joins string) scanner.ChannelView {
		c := base
		c.ID, c.Title, c.ChatType, c.Health, c.JoinRequests = id, title, typ, h, joins
		return c
	}
	s := &svc{
		channels: map[int64]scanner.ChannelView{
			-1001: mk(-1001, "Avito Product News", "channel", scanner.ChannelHealth{Status: "red", Reason: "violations", LastCheck: check(now().Add(-2*time.Hour), 3)}, "manual"),
			-1002: mk(-1002, "Команда платформы", "supergroup", scanner.ChannelHealth{Status: "green", Reason: "ok", LastCheck: check(now().Add(-5*time.Hour), 0)}, "auto"),
			-1003: mk(-1003, "Архив релизов", "channel", scanner.ChannelHealth{Status: "yellow", Reason: "stale", LastCheck: check(now().Add(-4*day), 0)}, "off"),
			-1004: mk(-1004, "Чужой канал, я только админ", "channel", scanner.ChannelHealth{Status: "yellow", Reason: "never_checked"}, "off"),
		},
		managers: map[int64]bool{-1001: true, -1002: true, -1003: true},
		wl:       map[int64]scanner.WhitelistEntryView{},
		avail:    []knownchats.Chat{{ID: -1009, Title: "Новый канал маркетинга", Type: "channel"}},
		joins: []scanner.JoinRequestView{
			{Request: joinrequests.Request{UserID: 21, FirstName: "Олег", Username: "oleg_emp", RequestedAt: now().Add(-3 * time.Hour), Check: "good", CheckedAt: now()}},
			{Request: joinrequests.Request{UserID: 22, FirstName: "Незнакомец", RequestedAt: now().Add(-2 * time.Hour), Check: "bad", CheckedAt: now(), Bio: "Crypto, trading"}},
			{Request: joinrequests.Request{UserID: 23, FirstName: "Без ника", RequestedAt: now().Add(-time.Hour), Check: "unknown", CheckedAt: now()}},
			{Request: joinrequests.Request{UserID: 24, FirstName: "Партнёр", Username: "partner", RequestedAt: now().Add(-30 * time.Minute), Check: "good", CheckedAt: now()}, Whitelisted: true},
		},
	}
	s.wl[42] = scanner.WhitelistEntryView{Entry: whitelist.Entry{UserID: 42, FirstName: "Подрядчик", Username: "contractor", ApprovedBy: 7, ApprovedAt: now().Add(-40 * day), ExpiresAt: now().Add(-10 * day), Note: "Агентство, договор №12"}, Expired: true}
	d := now().Add(5 * day)
	s.wl[43] = scanner.WhitelistEntryView{Entry: whitelist.Entry{UserID: 43, FirstName: "Стажёр", ApprovedBy: 7, ApprovedAt: now().Add(-2 * day), ExpiresAt: now().Add(28 * day), DeleteAt: &d, Note: "Летняя стажировка"}}
	s.events = []audit.Event{
		{At: now().Add(-time.Hour), Action: audit.ActionScanCompleted, Counts: map[string]int{"bad": 3, "good": 40, "whitelisted": 2}, Details: map[string]any{"source": "schedule"}},
		{At: now().Add(-2 * time.Hour), ActorID: 5, ActorName: "Anna Admin", Action: audit.ActionWhitelistAdded, Users: []audit.User{{ID: 43, Name: "Стажёр"}}, Note: "Летняя стажировка"},
		{At: now().Add(-3 * time.Hour), Action: audit.ActionJoinApproved, Users: []audit.User{{ID: 30, Name: "Мария"}}, Details: map[string]any{"auto": true}},
	}

	srv, err := webapi.New(s, auth{}, token, []byte("01234567890123456789012345678901"), webapi.WithMiniApp(s, auth{}))
	if err != nil {
		panic(err)
	}
	theme := url.QueryEscape(`{"bg_color":"#ffffff","secondary_bg_color":"#f1f2f5","text_color":"#111111","hint_color":"#8a8f98","button_color":"#2481cc","button_text_color":"#ffffff","link_color":"#2481cc","destructive_text_color":"#e53935"}`)
	http.HandleFunc("/start", func(w http.ResponseWriter, r *http.Request) {
		uid, _ := strconv.ParseInt(r.URL.Query().Get("uid"), 10, 64)
		if uid == 0 {
			uid = 5
		}
		http.Redirect(w, r, "/miniapp/#tgWebAppData="+url.QueryEscape(initData(uid))+"&tgWebAppVersion=8.0&tgWebAppPlatform=web&tgWebAppThemeParams="+theme, http.StatusFound)
	})
	http.Handle("/", srv.Handler())
	port := os.Getenv("PORT")
	if port == "" {
		port = "8797"
	}
	fmt.Println("listening :" + port)
	panic(http.ListenAndServe("127.0.0.1:"+port, nil))
}
