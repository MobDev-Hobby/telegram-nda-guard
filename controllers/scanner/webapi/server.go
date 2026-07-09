// Package webapi exposes the scanner's ManagementService over a JSON REST API
// and authenticates callers via the Telegram Login Widget.
//
// It depends only on two scanner interfaces — ManagementService and
// WebAuthenticator — plus the bot token (for verifying Telegram Login
// callbacks). It does NOT import the concrete *Domain, *guard.Update forms, or
// the storage layer, so the transport is fully substitutable.
package webapi

import (
	"embed"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/MobDev-Hobby/telegram-nda-guard/controllers/scanner"
)

//go:embed webui/*.html webui/*.js
var webuiFS embed.FS

// Logger mirrors the project-wide logger contract (per-package duplicate, as
// elsewhere in the codebase).
type Logger interface {
	Errorf(template string, args ...any)
	Warnf(template string, args ...any)
	Infof(template string, args ...any)
	Debugf(template string, args ...any)
}

// noopLogger discards all output.
type noopLogger struct{}

func (noopLogger) Errorf(string, ...any) {}
func (noopLogger) Warnf(string, ...any)  {}
func (noopLogger) Infof(string, ...any)  {}
func (noopLogger) Debugf(string, ...any) {}

// Server is the HTTP management API. Construct it with New and serve it with
// ListenAndServe (or wire ServeHTTP into your own *http.Server for custom
// timeouts/TLS).
type Server struct {
	service       scanner.ManagementService
	authenticator scanner.WebAuthenticator
	botToken      string // used to verify Telegram Login callbacks
	sessionSecret []byte // used to sign session cookies (>=32 bytes)
	cookieName    string
	cookieTTL     time.Duration
	log           Logger
	mux           *http.ServeMux
}

// Option configures a Server.
type Option func(*Server)

// WithLogger injects a logger.
func WithLogger(log Logger) Option {
	return func(s *Server) { s.log = log }
}

// WithCookieName overrides the session cookie name (default "tgndag_session").
func WithCookieName(name string) Option {
	return func(s *Server) { s.cookieName = name }
}

// WithCookieTTL overrides the session cookie lifetime (default 7 days).
func WithCookieTTL(ttl time.Duration) Option {
	return func(s *Server) { s.cookieTTL = ttl }
}

// New creates a Server. sessionSecret must be at least 32 bytes; it signs the
// session cookie so a stolen cookie cannot be forged without it. botToken is
// the Telegram bot token, used to verify Telegram Login Widget callbacks.
func New(
	service scanner.ManagementService,
	authenticator scanner.WebAuthenticator,
	botToken string,
	sessionSecret []byte,
	opts ...Option,
) (*Server, error) {
	if service == nil {
		return nil, errors.New("webapi: service is nil")
	}
	if authenticator == nil {
		return nil, errors.New("webapi: authenticator is nil")
	}
	if len(sessionSecret) < 32 {
		return nil, errors.New("webapi: sessionSecret must be at least 32 bytes")
	}
	if botToken == "" {
		return nil, errors.New("webapi: botToken is empty")
	}
	s := &Server{
		service:       service,
		authenticator: authenticator,
		botToken:      botToken,
		sessionSecret: sessionSecret,
		cookieName:    "tgndag_session",
		cookieTTL:     7 * 24 * time.Hour,
		log:           noopLogger{},
	}
	for _, opt := range opts {
		opt(s)
	}
	s.routes()
	return s, nil
}

// ListenAndServe starts the HTTP server on addr (e.g. ":8080"). It blocks
// until the server stops.
func (s *Server) ListenAndServe(addr string) error {
	s.log.Infof("webapi: listening on %s", addr)
	srv := &http.Server{
		Addr:              addr,
		Handler:           s.mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	return srv.ListenAndServe()
}

// routes wires every endpoint. Auth-protected routes go through s.requireAuth.
func (s *Server) routes() {
	s.mux = http.NewServeMux()
	// SPA static assets (login screen + dashboard), embedded into the binary.
	uiSub, _ := fs.Sub(webuiFS, "webui")
	s.mux.Handle("/", http.FileServer(http.FS(uiSub)))
	// API
	s.mux.HandleFunc("/api/auth/login", s.handleLogin)
	s.mux.HandleFunc("/api/auth/me", s.requireAuth(s.handleMe))
	s.mux.HandleFunc("/api/status", s.requireAuth(s.handleGetStatus))
	s.mux.HandleFunc("/api/channels", s.requireAuth(s.handleChannels))
	s.mux.HandleFunc("/api/channels/", s.requireAuth(s.handleChannelByPath)) // {id}, {id}/users, {id}/scan, {id}/clean
	s.mux.HandleFunc("/api/refresh-rights", s.requireAuth(s.handleRefreshRights))
}

// --- request/response helpers ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// scopeChatIDFrom extracts an optional ?chat= query param (the controlling chat
// for the operation), used by the WebAuthenticator's admin-check branch.
func scopeChatIDFrom(r *http.Request) int64 {
	q := r.URL.Query().Get("chat")
	if q == "" {
		return 0
	}
	id, err := strconv.ParseInt(q, 10, 64)
	if err != nil {
		return 0
	}
	return id
}

// parsePathID extracts the first path segment after prefix as an int64.
// E.g. parsePathID("/api/channels/123/users", "/api/channels/") -> 123, "users".
func parsePathID(path, prefix string) (int64, string, bool) {
	rest := strings.TrimPrefix(path, prefix)
	parts := strings.SplitN(rest, "/", 2)
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, "", false
	}
	sub := ""
	if len(parts) == 2 {
		sub = parts[1]
	}
	return id, sub, true
}
