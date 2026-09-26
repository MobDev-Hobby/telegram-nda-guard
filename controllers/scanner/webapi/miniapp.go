package webapi

import (
	"context"
	"errors"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/MobDev-Hobby/telegram-nda-guard/controllers/scanner"
)

// ChannelAuthorizer decides which channels a Mini App user may see.
// authorizer.HybridAuthorizer implements it.
type ChannelAuthorizer interface {
	// AuthorizeChannel reports whether callerID is an administrator of
	// channelID in Telegram (or privileged).
	AuthorizeChannel(ctx context.Context, callerID, channelID int64) (bool, error)
	// IsPrivileged reports whether callerID may manage every channel.
	IsPrivileged(ctx context.Context, callerID int64) bool
}

// Error codes the Mini App reacts to.
const (
	codeNotEmployee = "not_employee"
	codeNotJoined   = "not_joined"
	codeNotAdmin    = "not_admin"
)

// MiniAppChannel is a channel as listed for one Mini App user.
type MiniAppChannel struct {
	scanner.ChannelView
	// Joined is false for administrators who have not joined the channel in
	// the bot yet; they see it greyed out with a "Join" action.
	Joined bool `json:"joined"`
}

// WithMiniApp enables the Telegram Mini App: its page under /miniapp/ and its
// API under /api/miniapp/.
//
// Access: only users who pass the access checker may sign in. A user sees
// the protected channels they administer in Telegram; to manage one they
// join it in the bot first. The owner and the allowlist manage everything.
func WithMiniApp(service scanner.MiniAppService, channelAuth ChannelAuthorizer) Option {
	return func(s *Server) {
		s.miniApp = service
		s.channelAuth = channelAuth
	}
}

// WithMiniAppSessionTTL overrides how long a Mini App session lasts (default
// 1 hour). The employee check is repeated on every sign-in.
func WithMiniAppSessionTTL(ttl time.Duration) Option {
	return func(s *Server) { s.miniAppSession = ttl }
}

func (s *Server) miniAppRoutes() {
	if s.miniApp == nil || s.channelAuth == nil {
		return
	}
	uiSub, _ := fs.Sub(webuiFS, "webui/miniapp")
	s.mux.Handle("/miniapp/", noStore(http.StripPrefix("/miniapp/", http.FileServer(http.FS(uiSub)))))
	s.mux.HandleFunc("/api/miniapp/auth", s.handleMiniAppAuth)
	s.mux.HandleFunc("/api/miniapp/channels", s.requireSession(s.handleMiniAppChannels))
	s.mux.HandleFunc("/api/miniapp/channels/", s.requireSession(s.handleMiniAppChannel))
	s.mux.HandleFunc("/api/miniapp/available", s.requireSession(s.handleMiniAppAvailable))
	s.mux.HandleFunc("/api/miniapp/available/", s.requireSession(s.handleMiniAppConnect))
}

func writeErrorCode(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]string{"error": msg, "code": code})
}

// handleMiniAppAuth exchanges Telegram initData for a session token, for
// users who pass the access checker only.
func (s *Server) handleMiniAppAuth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body struct {
		InitData string `json:"initData"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	user, err := verifyWebAppInitData(s.botToken, body.InitData, webAppInitDataMaxAge, time.Now())
	if err != nil {
		s.log.Warnf("webapi: mini app initData rejected: %s", err)
		writeError(w, http.StatusUnauthorized, "invalid Telegram launch data")
		return
	}
	allowed, err := s.miniApp.CanUseMiniApp(r.Context(), user)
	if err != nil {
		s.log.Errorf("webapi: employee check of %d failed: %s", user.ID, err)
		writeError(w, http.StatusServiceUnavailable, "can't verify access right now, try again later")
		return
	}
	if !allowed {
		s.log.Warnf("webapi: mini app denied to %d (@%s): access check failed", user.ID, user.Username)
		writeErrorCode(w, http.StatusForbidden, codeNotEmployee, "the app is available to employees only")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"token":       s.newSessionTokenTTL(user.ID, s.miniAppSession, sessionMiniApp),
		"userId":      user.ID,
		"privileged":  s.channelAuth.IsPrivileged(r.Context(), user.ID),
		"botUsername": s.miniApp.BotUsername(),
	})
}

// handleMiniAppChannels lists the protected channels the caller administers,
// marking the ones they haven't joined in the bot.
func (s *Server) handleMiniAppChannels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	callerID := callerIDFromContext(r.Context())
	privileged := s.channelAuth.IsPrivileged(r.Context(), callerID)
	all, err := s.miniApp.ListChannels(r.Context(), 0)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]MiniAppChannel, 0, len(all))
	for _, ch := range all {
		ok, err := s.channelAuth.AuthorizeChannel(r.Context(), callerID, ch.ID)
		if err != nil {
			s.log.Errorf("webapi: authorize %d for %d: %s", callerID, ch.ID, err)
			continue
		}
		if ok {
			out = append(out, MiniAppChannel{
				ChannelView: ch,
				Joined:      privileged || s.miniApp.IsChannelManager(ch.ID, callerID),
			})
		}
	}
	writeJSON(w, http.StatusOK, out)
}

// handleMiniAppChannel routes /api/miniapp/channels/{id}[/...]:
//
//	POST {id}/join                                join the channel in the bot
//	GET  {id}                                     channel with its settings
//	PUT  {id}/settings                            replace all settings
//	POST {id}/scans                               start a scan (or join the running one)
//	GET  {id}/scans/{scanId}                      scan progress and result
//	POST {id}/scans/{scanId}/users/{userId}/recheck  run the checker again for one user
//	POST {id}/kick                                kick users selected from a scan
//	GET  {id}/audit                               action log, newest first
//	…    {id}/whitelist[/...]                     see handleMiniAppWhitelist
//
// Every route needs the caller to administer the channel in Telegram; all but
// join also need them to have joined it (or be privileged).
func (s *Server) handleMiniAppChannel(w http.ResponseWriter, r *http.Request) {
	channelID, sub, ok := parsePathID(r.URL.Path, "/api/miniapp/channels/")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid channel id")
		return
	}
	ctx := r.Context()
	callerID := callerIDFromContext(ctx)
	allowed, err := s.channelAuth.AuthorizeChannel(ctx, callerID, channelID)
	if err != nil || !allowed {
		writeErrorCode(w, http.StatusForbidden, codeNotAdmin, "you are not an administrator of this channel")
		return
	}

	if sub == "join" {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		if err := s.miniApp.JoinChannel(ctx, channelID, callerID); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		s.log.Infof("webapi: user %d joined %d", callerID, channelID)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		return
	}
	if !s.channelAuth.IsPrivileged(ctx, callerID) && !s.miniApp.IsChannelManager(channelID, callerID) {
		writeErrorCode(w, http.StatusForbidden, codeNotJoined, "join the channel in the bot first")
		return
	}

	switch {
	case sub == "" && r.Method == http.MethodGet:
		ch, err := s.miniApp.GetChannel(ctx, channelID)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, MiniAppChannel{ChannelView: ch, Joined: true})

	case sub == "settings" && r.Method == http.MethodPut:
		var settings scanner.ChannelSettings
		if err := decodeJSON(r, &settings); err != nil {
			writeError(w, http.StatusBadRequest, "invalid body")
			return
		}
		if err := s.miniApp.SetChannelSettings(ctx, channelID, settings, callerID); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		s.log.Infof("webapi: user %d changed settings of %d: %+v", callerID, channelID, settings)
		ch, err := s.miniApp.GetChannel(ctx, channelID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, MiniAppChannel{ChannelView: ch, Joined: true})

	case sub == "scans" && r.Method == http.MethodPost:
		scan, err := s.miniApp.StartChannelScan(ctx, channelID, callerID)
		if err != nil {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		writeJSON(w, http.StatusAccepted, scan)

	case strings.HasPrefix(sub, "scans/"):
		s.handleMiniAppScan(w, r, channelID, callerID, strings.TrimPrefix(sub, "scans/"))

	case sub == "kick" && r.Method == http.MethodPost:
		var body struct {
			ScanID  string  `json:"scanId"`
			UserIDs []int64 `json:"userIds"`
		}
		if err := decodeJSON(r, &body); err != nil || body.ScanID == "" || len(body.UserIDs) == 0 {
			writeError(w, http.StatusBadRequest, "scanId and userIds are required")
			return
		}
		results, err := s.miniApp.KickScannedUsers(ctx, channelID, body.ScanID, body.UserIDs, callerID)
		if err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, scanner.ErrScanNotFound) {
				status = http.StatusNotFound
			}
			writeError(w, status, err.Error())
			return
		}
		s.log.Infof("webapi: user %d kicked from %d: %+v", callerID, channelID, results)
		writeJSON(w, http.StatusOK, map[string]any{"results": results})

	case sub == "audit" && r.Method == http.MethodGet:
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		events, err := s.miniApp.ListAudit(ctx, channelID, limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, events)

	case sub == "joins" || strings.HasPrefix(sub, "joins/"):
		s.handleMiniAppJoins(w, r, channelID, callerID, strings.TrimPrefix(strings.TrimPrefix(sub, "joins"), "/"))

	case sub == "whitelist" || strings.HasPrefix(sub, "whitelist/"):
		s.handleMiniAppWhitelist(w, r, channelID, callerID, strings.TrimPrefix(strings.TrimPrefix(sub, "whitelist"), "/"))

	default:
		writeError(w, http.StatusNotFound, "unknown operation")
	}
}

// handleMiniAppScan routes {scanId} and {scanId}/users/{userId}/recheck.
func (s *Server) handleMiniAppScan(w http.ResponseWriter, r *http.Request, channelID, callerID int64, rest string) {
	scanID, tail, _ := strings.Cut(rest, "/")
	switch {
	case tail == "" && r.Method == http.MethodGet:
		scan, err := s.miniApp.GetChannelScan(r.Context(), channelID, scanID)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, scan)

	case strings.HasPrefix(tail, "users/") && strings.HasSuffix(tail, "/recheck") && r.Method == http.MethodPost:
		userID, err := strconv.ParseInt(strings.TrimSuffix(strings.TrimPrefix(tail, "users/"), "/recheck"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid user id")
			return
		}
		user, err := s.miniApp.RecheckScannedUser(r.Context(), channelID, scanID, userID, callerID)
		if err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, scanner.ErrScanNotFound) {
				status = http.StatusNotFound
			}
			writeError(w, status, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, user)

	default:
		writeError(w, http.StatusNotFound, "unknown operation")
	}
}

// handleMiniAppWhitelist routes /api/miniapp/channels/{id}/whitelist[/...]:
//
//	GET    whitelist                 entries, expired included
//	POST   whitelist                 approve users from a scan {scanId, userIds, note}
//	POST   whitelist/{userId}/renew  re-approve for another period {note?}
//	DELETE whitelist/{userId}        remove
func (s *Server) handleMiniAppWhitelist(w http.ResponseWriter, r *http.Request, channelID, callerID int64, rest string) {
	ctx := r.Context()
	switch {
	case rest == "" && r.Method == http.MethodGet:
		entries, err := s.miniApp.ListWhitelist(ctx, channelID)
		if err != nil {
			writeWhitelistError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, entries)

	case rest == "" && r.Method == http.MethodPost:
		var body struct {
			ScanID  string  `json:"scanId"`
			UserIDs []int64 `json:"userIds"`
			Note    string  `json:"note"`
			// TTLDays > 0 makes the approval temporary.
			TTLDays int `json:"ttlDays"`
		}
		if err := decodeJSON(r, &body); err != nil || body.ScanID == "" || len(body.UserIDs) == 0 {
			writeError(w, http.StatusBadRequest, "scanId and userIds are required")
			return
		}
		added, err := s.miniApp.AddToWhitelist(ctx, channelID, body.ScanID, body.UserIDs, body.Note, days(body.TTLDays), callerID)
		if err != nil {
			writeWhitelistError(w, err)
			return
		}
		s.log.Infof("webapi: user %d whitelisted %v in %d", callerID, body.UserIDs, channelID)
		writeJSON(w, http.StatusOK, added)

	default:
		userIDStr, action, _ := strings.Cut(rest, "/")
		userID, err := strconv.ParseInt(userIDStr, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid user id")
			return
		}
		switch {
		case action == "renew" && r.Method == http.MethodPost:
			var body struct {
				Note    string `json:"note"`
				TTLDays int    `json:"ttlDays"`
			}
			if r.ContentLength > 0 {
				if err := decodeJSON(r, &body); err != nil {
					writeError(w, http.StatusBadRequest, "invalid body")
					return
				}
			}
			entry, err := s.miniApp.RenewWhitelistEntry(ctx, channelID, userID, body.Note, days(body.TTLDays), callerID)
			if err != nil {
				writeWhitelistError(w, err)
				return
			}
			s.log.Infof("webapi: user %d re-approved %d in %d", callerID, userID, channelID)
			writeJSON(w, http.StatusOK, entry)
		case action == "" && r.Method == http.MethodDelete:
			if err := s.miniApp.RemoveWhitelistEntry(ctx, channelID, userID, callerID); err != nil {
				writeWhitelistError(w, err)
				return
			}
			s.log.Infof("webapi: user %d removed %d from whitelist of %d", callerID, userID, channelID)
			writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		default:
			writeError(w, http.StatusNotFound, "unknown operation")
		}
	}
}

func writeWhitelistError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, scanner.ErrWhitelistDisabled), errors.Is(err, scanner.ErrNotWhitelisted), errors.Is(err, scanner.ErrScanNotFound),
		errors.Is(err, scanner.ErrJoinRequestsDisabled), errors.Is(err, scanner.ErrJoinRequestNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	default:
		writeError(w, http.StatusBadRequest, err.Error())
	}
}

// noStore keeps Telegram's webview from serving a stale Mini App after a
// deploy.
func noStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

func days(n int) time.Duration {
	if n <= 0 {
		return 0
	}
	return time.Duration(n) * 24 * time.Hour
}

// handleMiniAppAvailable lists chats where the bot is an administrator, not
// yet protected, that the caller administers.
func (s *Server) handleMiniAppAvailable(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	callerID := callerIDFromContext(r.Context())
	chats, err := s.miniApp.ListAvailableChats(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := chats[:0]
	for _, c := range chats {
		if ok, err := s.channelAuth.AuthorizeChannel(r.Context(), callerID, c.ID); err == nil && ok {
			out = append(out, c)
		}
	}
	writeJSON(w, http.StatusOK, out)
}

// handleMiniAppConnect handles POST /api/miniapp/available/{id}/connect.
func (s *Server) handleMiniAppConnect(w http.ResponseWriter, r *http.Request) {
	chatID, sub, ok := parsePathID(r.URL.Path, "/api/miniapp/available/")
	if !ok || sub != "connect" || r.Method != http.MethodPost {
		writeError(w, http.StatusNotFound, "unknown operation")
		return
	}
	callerID := callerIDFromContext(r.Context())
	allowed, err := s.channelAuth.AuthorizeChannel(r.Context(), callerID, chatID)
	if err != nil || !allowed {
		writeErrorCode(w, http.StatusForbidden, codeNotAdmin, "you are not an administrator of this chat")
		return
	}
	if err := s.miniApp.ConnectChat(r.Context(), chatID, callerID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.log.Infof("webapi: user %d connected chat %d", callerID, chatID)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// handleMiniAppJoins routes /api/miniapp/channels/{id}/joins[/...]:
//
//	GET  joins                    pending join requests
//	POST joins/approve            approve {userIds}
//	POST joins/decline            decline {userIds}
//	POST joins/{userId}/recheck   run the checker again for one requester
func (s *Server) handleMiniAppJoins(w http.ResponseWriter, r *http.Request, channelID, callerID int64, rest string) {
	ctx := r.Context()
	switch {
	case rest == "" && r.Method == http.MethodGet:
		list, err := s.miniApp.ListJoinRequests(ctx, channelID)
		if err != nil {
			writeWhitelistError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, list)

	case (rest == "approve" || rest == "decline") && r.Method == http.MethodPost:
		var body struct {
			UserIDs []int64 `json:"userIds"`
		}
		if err := decodeJSON(r, &body); err != nil || len(body.UserIDs) == 0 {
			writeError(w, http.StatusBadRequest, "userIds are required")
			return
		}
		results, err := s.miniApp.ResolveJoinRequests(ctx, channelID, body.UserIDs, rest == "approve", callerID)
		if err != nil {
			writeWhitelistError(w, err)
			return
		}
		s.log.Infof("webapi: user %d %sd join requests %v in %d", callerID, rest, body.UserIDs, channelID)
		writeJSON(w, http.StatusOK, map[string]any{"results": results})

	case strings.HasSuffix(rest, "/recheck") && r.Method == http.MethodPost:
		userID, err := strconv.ParseInt(strings.TrimSuffix(rest, "/recheck"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid user id")
			return
		}
		view, err := s.miniApp.RecheckJoinRequest(ctx, channelID, userID, callerID)
		if err != nil {
			writeWhitelistError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, view)

	default:
		writeError(w, http.StatusNotFound, "unknown operation")
	}
}
