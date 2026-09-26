package webapi

import (
	"net/http"
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
