package webapi

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

// handleLogin verifies a Telegram Login Widget callback (form-encoded query
// params), authorizes the resulting user ID, and issues a session cookie on
// success.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	values := map[string]string{}
	for k, v := range r.URL.Query() {
		if len(v) > 0 {
			values[k] = v[0]
		}
	}
	uid, err := s.verifyTelegramLogin(values, 5*time.Minute)
	if err != nil {
		s.log.Warnf("webapi: login verification failed: %s", err)
		writeError(w, http.StatusUnauthorized, "login verification failed")
		return
	}
	// Authorize at the global scope (no chat context).
	allowed, err := s.authenticator.AuthenticateAndAuthorize(r.Context(), uid, 0)
	if err != nil || !allowed {
		writeError(w, http.StatusForbidden, "not authorized")
		return
	}
	s.issueSession(w, uid)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "userId": uid})
}

// handleMe returns the authenticated caller's user ID.
func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"userId": callerIDFromContext(r.Context())})
}

// handleGetStatus returns the aggregated bot/channel status.
func (s *Server) handleGetStatus(w http.ResponseWriter, r *http.Request) {
	status, err := s.service.GetStatus(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, status)
}

// handleChannels handles GET (list) and POST (add) on /api/channels.
func (s *Server) handleChannels(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		channels, err := s.service.ListChannels(r.Context(), scopeChatIDFrom(r))
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, channels)
	case http.MethodPost:
		var body struct {
			ID          int64 `json:"id"`
			CommandChat int64 `json:"commandChat"`
			AutoScan    bool  `json:"autoScan"`
			AutoClean   bool  `json:"autoClean"`
			AllowClean  bool  `json:"allowClean"`
		}
		if err := decodeJSON(r, &body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid body: "+err.Error())
			return
		}
		if body.ID == 0 || body.CommandChat == 0 {
			writeError(w, http.StatusBadRequest, "id and commandChat are required")
			return
		}
		if err := s.service.AddChannel(r.Context(), body.ID, body.CommandChat, body.AutoScan, body.AutoClean, body.AllowClean); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"ok": true})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleChannelByPath routes sub-paths of /api/channels/{id}:
//   GET    /api/channels/{id}           -> get channel
//   PATCH  /api/channels/{id}/flags     -> set flags
//   DELETE /api/channels/{id}           -> remove (?chat= required)
//   GET    /api/channels/{id}/users     -> list members
//   POST   /api/channels/{id}/scan      -> trigger scan (?chat= required)
//   POST   /api/channels/{id}/clean     -> trigger clean (?chat= required)
func (s *Server) handleChannelByPath(w http.ResponseWriter, r *http.Request) {
	id, sub, ok := parsePathID(r.URL.Path, "/api/channels/")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid channel id")
		return
	}
	switch sub {
	case "":
		if r.Method == http.MethodGet {
			ch, err := s.service.GetChannel(r.Context(), id)
			if err != nil {
				writeError(w, http.StatusNotFound, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, ch)
			return
		}
		if r.Method == http.MethodDelete {
			chat := scopeChatIDFrom(r)
			if chat == 0 {
				writeError(w, http.StatusBadRequest, "?chat= is required to remove a channel")
				return
			}
			if err := s.service.RemoveChannel(r.Context(), id, chat); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"ok": true})
			return
		}
	case "flags":
		if r.Method != http.MethodPatch {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		var body struct {
			AutoScan   bool `json:"autoScan"`
			AutoClean  bool `json:"autoClean"`
			AllowClean bool `json:"allowClean"`
		}
		if err := decodeJSON(r, &body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid body: "+err.Error())
			return
		}
		if err := s.service.SetChannelFlags(r.Context(), id, body.AutoScan, body.AutoClean, body.AllowClean); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
		return
	case "users":
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		users, err := s.service.ListChannelUsers(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, users)
		return
	case "scan":
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		chat := scopeChatIDFrom(r)
		if chat == 0 {
			writeError(w, http.StatusBadRequest, "?chat= is required to trigger a scan")
			return
		}
		if err := s.service.TriggerScan(r.Context(), id, chat); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]any{"ok": true})
		return
	case "clean":
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		chat := scopeChatIDFrom(r)
		if chat == 0 {
			writeError(w, http.StatusBadRequest, "?chat= is required to trigger a clean")
			return
		}
		if err := s.service.TriggerClean(r.Context(), id, chat); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]any{"ok": true})
		return
	}
	writeError(w, http.StatusNotFound, "unknown operation")
}

// handleRefreshRights re-resolves bot rights from Telegram.
func (s *Server) handleRefreshRights(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if err := s.service.RefreshRights(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// decodeJSON decodes a JSON request body into dst with a sane size limit.
func decodeJSON(r *http.Request, dst any) error {
	defer r.Body.Close()
	r.Body = http.MaxBytesReader(nil, r.Body, 1<<16) // 64 KiB
	return json.NewDecoder(r.Body).Decode(dst)
}

// _ keeps strconv referenced for potential numeric query parsing extensions.
var _ = strconv.Atoi
