package o11y

// anonymous.go answers ONE question about this package's route table: does the
// op at method+path answer a caller who holds no Hanzo principal?
//
// WHY THE ANSWER LIVES HERE. It is a fact about these routes, so it belongs
// beside them. It used to live in the embedding host instead — hanzoai/cloud's
// apps/o11y kept its own list of "the o11y paths that need no principal" — and
// that list named /v1/o11y/api/v1/health, /api/v2/healthz, /api/v2/readyz and
// /api/v2/livez: the INTERNAL namespace this module used to rewrite onto before
// PATH UNTOUCHED (mount.go) removed the rewrite. The paths it named stopped
// existing and the paths that exist were never named, so the host's exemption
// matched nothing at all and every public op — /version, /health, the three
// probes, sign-in, the shared-dashboard reads — was refused 403 at the unified
// door while the standalone door served them 200. A copy of a route fact, kept
// one repo away from the routes, drifted the moment the routes moved. There is
// one copy now and it is this one.
//
// WHAT THE HOST'S GATE IS FOR, stated so the exemption can be argued rather
// than enumerated. The runtime trusts X-Org-Id as gateway-minted: whoever sets
// that header names the tenant whose telemetry is read. Off-gateway — a direct
// pod call, a bearer-less request — the identity middleware restores a client's
// X-Org-Id but mints no X-User-Id, so the host refuses a request that carries a
// tenant and no principal before the runtime can believe it.
//
// THE RULE, therefore, in one sentence: an op whose own gate reads no tenant
// from the request has nothing for a forged tenant to reach, so gating it can
// only remove an answer. Anonymous is exactly that set — the runtime's
// OpenAccess routes plus the two public-dashboard reads it gates with
// CheckWithoutClaims — and nothing else. Every other op stays gated at the host
// AND at the runtime, which is one rule enforced twice, not two rules.
//
// Anonymous does NOT authorize. It says a principal is not the ADMISSION test;
// the op's own credential still is — a DSN key on the ingest wires, a
// service-account key on service_accounts/me, the share's scope on a public
// dashboard, a session cookie on /users/me (which answers 401 without one, and
// that is the honest answer rather than the host's 403 for a route the runtime
// would have served).

import (
	"net/http"
	"strings"
)

// Anonymous reports whether the op at method+path is served to a caller with no
// Hanzo principal. Path is the FULL public path as it arrives on the wire —
// this module never rewrites one (mount.go), so the route literal and the
// argument are the same spelling.
//
// anonymous_test.go proves every static entry below is a route Mount actually
// registers, which is the drift that broke the unified door: a path in an
// exemption list that no route serves.
func Anonymous(method, path string) bool {
	if openOps[method+" "+path] {
		return true
	}
	return publicDashboardRead(method, path) || IngestWire(method, path)
}

// openOps is every FIXED-path op the runtime serves without a principal. Each
// line is one of its OpenAccess registrations; the grouping is the reason.
var openOps = map[string]bool{
	// The process describing ITSELF. No tenant is read and none could be: a
	// kubelet probe and an external status check hold no principal, and a gate
	// that hides whether the process is up makes an incident invisible at
	// exactly the moment it matters. /version and /health are also the two the
	// console reads before it has a session, to know what it is talking to.
	"GET /v1/o11y/livez":   true,
	"GET /v1/o11y/healthz": true,
	"GET /v1/o11y/readyz":  true,
	"GET /v1/o11y/version": true,
	"GET /v1/o11y/health":  true,

	// The console's pre-login configuration — branding, the ingest URL, which
	// sign-in methods this deployment offers. Read to RENDER the sign-in page,
	// so requiring a session to read it is circular.
	"GET /v1/o11y/global/config": true,

	// THE PATH TO HAVING A PRINCIPAL. Requiring one to obtain one is the same
	// circle, and the failure it causes is total: sign-in itself stops working.
	// Registration is here because on a fresh deployment there is no user yet to
	// hold a principal; the runtime refuses it once setup has completed.
	"POST /v1/o11y/register":                     true,
	"POST /v1/o11y/sessions/email_password":      true,
	"GET /v1/o11y/sessions/context":              true,
	"POST /v1/o11y/sessions/rotate":              true,
	"DELETE /v1/o11y/sessions":                   true,
	"GET /v1/o11y/complete/google":               true,
	"GET /v1/o11y/complete/oidc":                 true,
	"POST /v1/o11y/complete/saml":                true,
	"POST /v1/o11y/reset_password_tokens/verify": true,
	"POST /v1/o11y/resetPassword":                true,
	"POST /v1/o11y/factor_password/forgot":       true,

	// SELF-ADDRESSED: the subject is the caller's own credential, never a tenant
	// named in the request. There is no X-Org-Id to forge into these — "me" is
	// read from the claims the runtime resolved — so the host has nothing to
	// protect and a caller with no claims gets the runtime's own 401.
	"GET /v1/o11y/user/me":                  true,
	"GET /v1/o11y/users/me":                 true,
	"PUT /v1/o11y/users/me":                 true,
	"PUT /v1/o11y/users/me/factor_password": true,
	"GET /v1/o11y/service_accounts/me":      true,
	"PUT /v1/o11y/service_accounts/me":      true,
}

// publicDashboardRead reports whether path is one of the two reads a SHARED
// dashboard's public page makes: the sanitized dashboard and one widget's data.
// Anonymous is the entire point of a share link, and the runtime gates both with
// CheckWithoutClaims scoped to that dashboard's own read scope — so the tenant
// comes from the share, not from a header a caller could set.
//
//	GET /v1/o11y/public/dashboards/{id}
//	GET /v1/o11y/public/dashboards/{id}/widgets/{idx}/query_range
func publicDashboardRead(method, path string) bool {
	if method != http.MethodGet {
		return false
	}
	rest, ok := strings.CutPrefix(path, o11yRoot+"/public/dashboards/")
	if !ok {
		return false
	}
	seg := strings.Split(rest, "/")
	switch len(seg) {
	case 1: // {id}
		return seg[0] != ""
	case 4: // {id}/widgets/{idx}/query_range
		return seg[0] != "" && seg[1] == "widgets" && seg[2] != "" && seg[3] == "query_range"
	}
	return false
}

// IngestWire reports whether method+path is a Sentry-compatible error-ingest
// WRITE — the one foreign protocol this surface RECEIVES. The caller is a Sentry
// SDK presenting a DSN public key, not a Hanzo session, and the runtime verifies
// that key itself (constant-time HMAC, fail-closed) and derives the org from the
// project segment. Refusing it for lacking a principal it can never carry would
// take every SDK in the field off the air.
//
//	POST /v1/o11y/api/{project}/envelope|store  (the DSN path an SDK appends to)
//	POST /v1/sentinel/{project}/envelope|store    (the same wire on the clean root)
//
// It matches by METHOD + PREFIX + SUFFIX and never by a bare prefix, so the read
// APIs under both roots — issues, discover, logs, traces, stats — stay gated.
// The trailing slash is the protocol's form; the slash-less variant is tolerated
// defensively. Exported because the EDGE needs the identical answer: the gateway
// waives its JWT check on exactly these, and a request the gateway lets through
// tokenless must not then be refused here for having no token.
func IngestWire(method, path string) bool {
	if method != http.MethodPost {
		return false
	}
	if !strings.HasPrefix(path, o11yRoot+"/api/") && !strings.HasPrefix(path, sentinelRoot+"/") {
		return false
	}
	return strings.HasSuffix(path, "/envelope/") || strings.HasSuffix(path, "/envelope") ||
		strings.HasSuffix(path, "/store/") || strings.HasSuffix(path, "/store")
}
