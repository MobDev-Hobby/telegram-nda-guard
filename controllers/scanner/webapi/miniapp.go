package webapi

import (
	"context"
	"errors"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/MobDev-Hobby/telegram-nda-guard/controllers/scanner"
)

// ChannelAuthorizer decides which channels a Mini App user may manage.
// authorizer.HybridAuthorizer implements it.
type ChannelAuthorizer interface {
	// AuthorizeChannel reports whether callerID may manage channelID.
	AuthorizeChannel(ctx context.Context, callerID, channelID int64) (bool, error)
	// IsPrivileged reports whether callerID may manage every channel.
	IsPrivileged(ctx context.Context, callerID int64) bool
}

// WithMiniApp enables the Telegram Mini App: its page under /miniapp/ and its
// API under /api/miniapp/. Unlike the dashboard, which is restricted to the
// globally authorized users, the Mini App lets every channel administrator
// manage the channels they administer.
func WithMiniApp(service scanner.MiniAppService, channelAuth ChannelAuthorizer) Option {
	return func(s *Server) {
		s.miniApp = service
		s.channelAuth = channelAuth
	}
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
}

// handleMiniAppAuth exchanges Telegram initData for a session token.
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
	uid, err := verifyWebAppInitData(s.botToken, body.InitData, webAppInitDataMaxAge, time.Now())
	if err != nil {
		s.log.Warnf("webapi: mini app initData rejected: %s", err)
		writeError(w, http.StatusUnauthorized, "invalid Telegram launch data")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"token":      s.newSessionToken(uid),
		"userId":     uid,
		"privileged": s.channelAuth.IsPrivileged(r.Context(), uid),
	})
}

// handleMiniAppChannels lists the channels the caller may manage.
func (s *Server) handleMiniAppChannels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	callerID := callerIDFromContext(r.Context())
	all, err := s.miniApp.ListChannels(r.Context(), 0)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]scanner.ChannelView, 0, len(all))
	for _, ch := range all {
		ok, err := s.channelAuth.AuthorizeChannel(r.Context(), callerID, ch.ID)
		if err != nil {
			s.log.Errorf("webapi: authorize %d for %d: %s", callerID, ch.ID, err)
			continue
		}
		if ok {
			out = append(out, ch)
		}
	}
	writeJSON(w, http.StatusOK, out)
}

// handleMiniAppChannel routes /api/miniapp/channels/{id}[/...]:
//
//	GET  {id}                 channel with its settings
//	PUT  {id}/settings        replace all settings
//	POST {id}/scans           start a scan (or join the running one)
//	GET  {id}/scans/{scanId}  scan progress and result
//	POST {id}/kick            kick users selected from a scan
func (s *Server) handleMiniAppChannel(w http.ResponseWriter, r *http.Request) {
	channelID, sub, ok := parsePathID(r.URL.Path, "/api/miniapp/channels/")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid channel id")
		return
	}
	callerID := callerIDFromContext(r.Context())
	allowed, err := s.channelAuth.AuthorizeChannel(r.Context(), callerID, channelID)
	if err != nil || !allowed {
		writeError(w, http.StatusForbidden, "you are not an administrator of this channel")
		return
	}

	switch {
	case sub == "" && r.Method == http.MethodGet:
		ch, err := s.miniApp.GetChannel(r.Context(), channelID)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, ch)

	case sub == "settings" && r.Method == http.MethodPut:
		var settings scanner.ChannelSettings
		if err := decodeJSON(r, &settings); err != nil {
			writeError(w, http.StatusBadRequest, "invalid body")
			return
		}
		if err := s.miniApp.SetChannelSettings(r.Context(), channelID, settings); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		s.log.Infof("webapi: user %d changed settings of %d: %+v", callerID, channelID, settings)
		ch, err := s.miniApp.GetChannel(r.Context(), channelID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, ch)

	case sub == "scans" && r.Method == http.MethodPost:
		scan, err := s.miniApp.StartChannelScan(r.Context(), channelID, callerID)
		if err != nil {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		writeJSON(w, http.StatusAccepted, scan)

	case strings.HasPrefix(sub, "scans/") && r.Method == http.MethodGet:
		scan, err := s.miniApp.GetChannelScan(r.Context(), channelID, strings.TrimPrefix(sub, "scans/"))
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, scan)

	case sub == "kick" && r.Method == http.MethodPost:
		var body struct {
			ScanID  string  `json:"scanId"`
			UserIDs []int64 `json:"userIds"`
		}
		if err := decodeJSON(r, &body); err != nil || body.ScanID == "" || len(body.UserIDs) == 0 {
			writeError(w, http.StatusBadRequest, "scanId and userIds are required")
			return
		}
		results, err := s.miniApp.KickScannedUsers(r.Context(), channelID, body.ScanID, body.UserIDs, callerID)
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

	default:
		writeError(w, http.StatusNotFound, "unknown operation")
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
