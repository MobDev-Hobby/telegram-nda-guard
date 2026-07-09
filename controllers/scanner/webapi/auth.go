package webapi

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// sessionPayload is the on-the-wire form of the session cookie: callerID plus a
// 64-bit expiry (unix seconds). It is MAC'd with sessionSecret so it cannot be
// forged or tampered with. Layout: [8 bytes callerID][8 bytes expiry][32 bytes hmac].
const sessionLen = 8 + 8 + 32

type contextKey string

const callerIDKey contextKey = "callerID"

// callerIDFromContext returns the authenticated caller's Telegram user ID, or 0
// if absent (the requireAuth middleware never allows that path to run).
func callerIDFromContext(ctx context.Context) int64 {
	if v, ok := ctx.Value(callerIDKey).(int64); ok {
		return v
	}
	return 0
}

// verifyTelegramLogin validates a Telegram Login Widget callback's hash against
// the bot token and checks the auth_date is within maxAge. On success it returns
// the verified user ID. See https://core.telegram.org/widgets/login#checking-authorization
func (s *Server) verifyTelegramLogin(values map[string]string, maxAge time.Duration) (int64, error) {
	gotHash := values["hash"]
	if gotHash == "" {
		return 0, errors.New("missing hash")
	}
	authDateStr := values["auth_date"]
	if authDateStr == "" {
		return 0, errors.New("missing auth_date")
	}
	authDate, err := strconv.ParseInt(authDateStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("bad auth_date: %w", err)
	}
	if maxAge > 0 && time.Since(time.Unix(authDate, 0)) > maxAge {
		return 0, errors.New("login data expired")
	}

	// Data-check string = sorted "k=v" pairs excluding "hash", joined by \n.
	parts := make([]string, 0, len(values))
	for k, v := range values {
		if k == "hash" {
			continue
		}
		parts = append(parts, k+"="+v)
	}
	// Sort for a stable data-check string.
	for i := 0; i < len(parts); i++ {
		for j := i + 1; j < len(parts); j++ {
			if parts[j] < parts[i] {
				parts[i], parts[j] = parts[j], parts[i]
			}
		}
	}
	dataCheckString := strings.Join(parts, "\n")

	// secret_key = sha256(bot_token); data_hash = hmac_sha256(data_check, secret_key).
	secret := sha256.Sum256([]byte(s.botToken))
	mac := hmac.New(sha256.New, secret[:])
	mac.Write([]byte(dataCheckString))
	wantHash := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(gotHash), []byte(wantHash)) {
		return 0, errors.New("invalid hash")
	}

	uid, err := strconv.ParseInt(values["id"], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("bad id: %w", err)
	}
	return uid, nil
}

// issueSession builds and sets a signed session cookie for callerID.
func (s *Server) issueSession(w http.ResponseWriter, callerID int64) {
	payload := make([]byte, sessionLen)
	binary.BigEndian.PutUint64(payload[0:8], uint64(callerID))
	binary.BigEndian.PutUint64(payload[8:16], uint64(time.Now().Add(s.cookieTTL).Unix()))
	mac := hmac.New(sha256.New, s.sessionSecret)
	mac.Write(payload[0:16])
	copy(payload[16:], mac.Sum(nil))

	http.SetCookie(w, &http.Cookie{
		Name:     s.cookieName,
		Value:    hex.EncodeToString(payload),
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(s.cookieTTL),
		MaxAge:   int(s.cookieTTL.Seconds()),
	})
}

// validateSession parses and verifies the session cookie, returning the caller
// ID and whether the session is still valid.
func (s *Server) validateSession(r *http.Request) (int64, bool) {
	c, err := r.Cookie(s.cookieName)
	if err != nil {
		return 0, false
	}
	raw, err := hex.DecodeString(c.Value)
	if err != nil || len(raw) != sessionLen {
		return 0, false
	}
	mac := hmac.New(sha256.New, s.sessionSecret)
	mac.Write(raw[0:16])
	if !hmac.Equal(raw[16:], mac.Sum(nil)) {
		return 0, false
	}
	expiry := int64(binary.BigEndian.Uint64(raw[8:16]))
	if time.Now().Unix() > expiry {
		return 0, false
	}
	return int64(binary.BigEndian.Uint64(raw[0:8])), true
}

// clearSession removes the session cookie.
func (s *Server) clearSession(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: s.cookieName, Value: "", Path: "/", MaxAge: -1, Expires: time.Unix(0, 0),
	})
}

// requireAuth wraps a handler: it validates the session cookie and, if present,
// asks the WebAuthenticator to authorize the caller for the request's scope
// chat. Authorized callers' IDs are placed in the request context.
func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		callerID, ok := s.validateSession(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		allowed, err := s.authenticator.AuthenticateAndAuthorize(r.Context(), callerID, scopeChatIDFrom(r))
		if err != nil {
			s.log.Errorf("webapi: auth error for %d: %s", callerID, err)
			writeError(w, http.StatusForbidden, "authorization error")
			return
		}
		if !allowed {
			writeError(w, http.StatusForbidden, "forbidden")
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), callerIDKey, callerID)))
	}
}
