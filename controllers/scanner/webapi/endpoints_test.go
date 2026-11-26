package webapi

import (
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Every listed endpoint must reach a handler (no 404 "unknown operation", no
// 405), so allowlists built from MiniAppEndpoints never miss a route.
func TestMiniAppEndpointsAreRouted(t *testing.T) {
	s, _ := newMiniAppServer(t)
	bodies := map[string]string{
		"POST /api/miniapp/auth":                     `{"initData":""}`,
		"PUT /api/miniapp/channels/*/settings":       `{}`,
		"POST /api/miniapp/channels/*/kick":          `{"scanId":"s1","userIds":[1]}`,
		"POST /api/miniapp/channels/*/whitelist":     `{"scanId":"s1","userIds":[1]}`,
		"POST /api/miniapp/channels/*/joins/approve": `{"userIds":[8]}`,
		"POST /api/miniapp/channels/*/joins/decline": `{"userIds":[8]}`,
	}
	for _, ep := range MiniAppEndpoints {
		method, pattern, _ := strings.Cut(ep, " ")
		path := concretePath(pattern)
		rec := miniAppRequest(s, 42, method, path, bodies[ep])
		assert.NotEqual(t, http.StatusMethodNotAllowed, rec.Code, ep)
		if rec.Code == http.StatusNotFound {
			assert.NotContains(t, rec.Body.String(), "unknown operation", ep)
			assert.NotContains(t, rec.Body.String(), "404 page not found", ep)
		}
	}
}

// concretePath fills "*" segments with values the fake service accepts.
func concretePath(pattern string) string {
	parts := strings.Split(pattern, "/")
	for i, p := range parts {
		if p != "*" {
			continue
		}
		switch parts[i-1] {
		case "miniapp":
			parts[i] = "app.js"
		case "scans":
			parts[i] = "s1"
		case "channels", "available":
			parts[i] = "-100"
		default:
			parts[i] = "11"
		}
	}
	return strings.Join(parts, "/")
}

// The Mini App routes by hand-written switches over the path tail, not by mux
// patterns, so nothing structural stops a new sub-path from being served
// while MiniAppEndpoints stays unchanged. That drift is invisible locally and
// breaks only in a deployment that builds a path allowlist from the list: the
// route answers 403 in production and nowhere else. It happened once, to
// "{id}/schedule". This reads the package's own sources and requires every
// segment the dispatchers compare against to appear in the list.
func TestMiniAppEndpointsListEveryRoutedSegment(t *testing.T) {
	listed := map[string]bool{}
	for _, ep := range MiniAppEndpoints {
		_, path, _ := strings.Cut(ep, " ")
		for _, seg := range strings.Split(path, "/") {
			if seg != "" && seg != "*" {
				listed[seg] = true
			}
		}
	}

	sources, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob sources: %v", err)
	}
	// "sub", "tail", "rest" and "action" are the path-tail variables the
	// dispatchers in this package switch on.
	compare := regexp.MustCompile(`\b(?:sub|tail|rest|action)\s*==\s*"([^"]*)"`)
	prefix := regexp.MustCompile(`strings\.Has(?:Prefix|Suffix)\((?:sub|tail|rest|action),\s*"([^"]*)"\)`)

	for _, src := range sources {
		if strings.HasSuffix(src, "_test.go") {
			continue
		}
		raw, readErr := os.ReadFile(src)
		if readErr != nil {
			t.Fatalf("read %s: %v", src, readErr)
		}
		for _, m := range append(
			compare.FindAllStringSubmatch(string(raw), -1),
			prefix.FindAllStringSubmatch(string(raw), -1)...,
		) {
			for _, seg := range strings.Split(m[1], "/") {
				if seg == "" {
					continue // "" is the collection route, already listed
				}
				assert.True(t, listed[seg],
					"%s routes %q, but no MiniAppEndpoints entry contains that segment; "+
						"deployments build their auth allowlist from that list", src, seg)
			}
		}
	}
}
