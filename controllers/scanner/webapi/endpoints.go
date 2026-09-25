package webapi

// MiniAppEndpoints lists every route the Mini App serves, as "METHOD /path"
// with "*" standing for exactly one path segment. Deployments that put the
// Mini App behind a path allowlist (e.g. a platform auth policy or an nginx
// location) can check their config against it; a test here keeps the list in
// sync with the router.
var MiniAppEndpoints = []string{
	"GET /miniapp/",
	"GET /miniapp/*",
	"POST /api/miniapp/auth",
	"GET /api/miniapp/channels",
	"GET /api/miniapp/available",
	"POST /api/miniapp/available/*/connect",
	"GET /api/miniapp/channels/*",
	"POST /api/miniapp/channels/*/join",
	"PUT /api/miniapp/channels/*/settings",
	"POST /api/miniapp/channels/*/scans",
	"GET /api/miniapp/channels/*/scans/*",
	"POST /api/miniapp/channels/*/scans/*/users/*/recheck",
	"POST /api/miniapp/channels/*/kick",
	"GET /api/miniapp/channels/*/audit",
	"GET /api/miniapp/channels/*/whitelist",
	"POST /api/miniapp/channels/*/whitelist",
	"POST /api/miniapp/channels/*/whitelist/*/renew",
	"DELETE /api/miniapp/channels/*/whitelist/*",
	"GET /api/miniapp/channels/*/joins",
	"POST /api/miniapp/channels/*/joins/approve",
	"POST /api/miniapp/channels/*/joins/decline",
	"POST /api/miniapp/channels/*/joins/*/recheck",
}
