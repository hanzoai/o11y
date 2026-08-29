package o11y_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/hanzoai/o11y"
)

// fixedExemptions mirrors anonymous.go's openOps table, spelled a second time
// ON PURPOSE: the assertion below is "every one of these is a real route", and
// a list derived from the thing under test could not make it. Adding to openOps
// without adding here fails TestEveryExemptOpIsNamed.
var fixedExemptions = []string{
	"GET /v1/o11y/livez",
	"GET /v1/o11y/healthz",
	"GET /v1/o11y/readyz",
	"GET /v1/o11y/version",
	"GET /v1/o11y/health",
	"GET /v1/o11y/global/config",
	"POST /v1/o11y/register",
	"POST /v1/o11y/sessions/email_password",
	"GET /v1/o11y/sessions/context",
	"POST /v1/o11y/sessions/rotate",
	"DELETE /v1/o11y/sessions",
	"GET /v1/o11y/complete/google",
	"GET /v1/o11y/complete/oidc",
	"POST /v1/o11y/complete/saml",
	"POST /v1/o11y/reset_password_tokens/verify",
	"POST /v1/o11y/resetPassword",
	"POST /v1/o11y/factor_password/forgot",
	"GET /v1/o11y/user/me",
	"GET /v1/o11y/users/me",
	"PUT /v1/o11y/users/me",
	"PUT /v1/o11y/users/me/factor_password",
	"GET /v1/o11y/service_accounts/me",
	"PUT /v1/o11y/service_accounts/me",
}

// THE DRIFT GUARD. The unified binary refused every public o11y op — /version,
// /health, the three probes, sign-in — because the embedding host's exemption
// list named /v1/o11y/api/v2/livez and three siblings under an internal /api/
// namespace this module had stopped rewriting onto. Four names, zero routes,
// and nothing anywhere said so. This test is what says so: every path the
// exemption names must be a route Mount registers, at that method.
//
// It is the arithmetic routes_test.go runs over the whole surface, applied to
// the one list allowed to name a subset of it.
func TestAnonymousNamesOnlyRealRoutes(t *testing.T) {
	have := registered(t, mounted(t))
	for _, route := range fixedExemptions {
		method, path, ok := strings.Cut(route, " ")
		if !ok {
			t.Fatalf("malformed exemption %q", route)
		}
		if !o11y.Anonymous(method, path) {
			t.Errorf("%q is listed here but Anonymous says false — this table and anonymous.go disagree", route)
		}
		if !have[route] {
			t.Errorf("Anonymous exempts %q, which Mount does not register — an exemption matching no route is how the unified binary lost its public ops", route)
		}
	}
}

// TestEveryExemptOpIsNamed is the converse: no route may be exempt without
// being declared above, where it can be reviewed and counted. The two
// parameterized families are exempt by SHAPE rather than by name — the census
// in routes_test.go counts them and the tables below pin them.
func TestEveryExemptOpIsNamed(t *testing.T) {
	named := map[string]bool{}
	for _, route := range fixedExemptions {
		named[route] = true
	}
	for route := range registered(t, mounted(t)) {
		method, path, _ := strings.Cut(route, " ")
		switch {
		case !o11y.Anonymous(method, path), named[route]:
		case strings.HasPrefix(path, "/v1/o11y/public/dashboards/"), o11y.IngestWire(method, path):
		default:
			t.Errorf("%s is exempt from the principal gate but is not named in fixedExemptions", route)
		}
	}
}

// A public op that stops being exempt is a 403 on the front page — the outage
// this whole file exists to prevent recurring.
func TestPublicOpsAreExempt(t *testing.T) {
	for _, route := range []string{
		"GET /v1/o11y/version",
		"GET /v1/o11y/health",
		"GET /v1/o11y/livez",
		"GET /v1/o11y/healthz",
		"GET /v1/o11y/readyz",
		"GET /v1/o11y/global/config",
		"POST /v1/o11y/sessions/email_password",
		"GET /v1/o11y/public/dashboards/d1",
		"GET /v1/o11y/public/dashboards/d1/widgets/0/query_range",
		"POST /v1/event/proj/envelope/",
		"POST /v1/event/proj/store",
		"POST /v1/o11y/api/proj/envelope/",
		"POST /v1/o11y/api/proj/store",
	} {
		method, path, _ := strings.Cut(route, " ")
		if !o11y.Anonymous(method, path) {
			t.Errorf("Anonymous(%s) = false, want true — this op serves callers who hold no principal", route)
		}
	}
}

// The gate is still a gate. Telemetry is a tenant's own data; if these ever
// answer true the route is open, which is worse than the 403 it replaced.
func TestTenantOpsStayGated(t *testing.T) {
	for _, route := range []string{
		"GET /v1/o11y/logs",
		"GET /v1/o11y/dashboards",
		"GET /v1/o11y/features",
		"GET /v1/o11y/service_accounts",    // the LIST, not /me
		"GET /v1/o11y/users",               // the roster, not /users/me
		"GET /v1/o11y/public/dashboards",   // the collection is not a share
		"GET /v1/o11y/public/dashboards//", // an empty share id
		"GET /v1/o11y/public/dashboards/d1/widgets/0/query_rangex",
		"GET /v1/event/proj/envelope/",  // ingest is a WRITE; a GET is not it
		"POST /v1/o11y/api/v3/issues",   // a read API under the DSN prefix
		"POST /v1/o11y/sentinel/issues", // the face is a face, whatever the suffix
		// Ingest moved off the face. The old spelling must buy nothing, or
		// the move left an unauthenticated hole where the route used to be.
		"POST /v1/o11y/sentinel/proj/envelope/",
		"POST /v1/o11y/sentinel/proj/store",
		"POST /v1/o11y/api/proj/envelopes",
		"POST /v1/event",        // the product event endpoint is not the Sentry wire
		"GET /v1/o11y/version/", // a route that does not exist is not public
		"GET /V1/O11Y/VERSION",  // and the match is exact, never folded
	} {
		method, path, _ := strings.Cut(route, " ")
		if o11y.Anonymous(method, path) {
			t.Errorf("Anonymous(%s) = true, want false — this op must keep its gate", route)
		}
	}
}

// The exemption is a fact about the ROUTE, not about the caller's method: a
// probe path answered for a write would be a hole with a health check's name.
func TestExemptionIsPerMethod(t *testing.T) {
	for _, m := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch} {
		if o11y.Anonymous(m, "/v1/o11y/version") {
			t.Errorf("Anonymous(%s /v1/o11y/version) = true, want false", m)
		}
	}
}
