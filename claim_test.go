package o11y_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hanzoai/o11y"
	"github.com/zap-proto/zip"
)

// What a claim has to be true of: the claimed address is NOT declared, every
// other address still is, and a host that claims nothing sees the whole table.
// A seam that skipped more than it was asked to would delete operations
// silently, which is the failure the wildcard used to hide.

// declaredAt reports whether this METHOD and path is in the table. With no
// runtime behind it a declared route answers 503
// (TestNamedRouteWithoutRuntimeReturns503), so the two refusals below are
// unambiguous — neither can be a handler's own answer.
//
// Both refusals count as "not declared", and the 405 is the interesting one: an
// address is a method AND a path, so claiming POST /query_range while GET
// /query_range stays declared leaves the PATH routable and the method not. The
// router says 405 to that, not 404, and reading only 404 would have called the
// claim a failure when it had done exactly what it was asked.
func declaredAt(t *testing.T, app *zip.App, method, target string) bool {
	t.Helper()
	resp, err := app.Test(httptest.NewRequest(method, target, nil))
	if err != nil {
		t.Fatalf("Test %s %s: %v", method, target, err)
	}
	return resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusMethodNotAllowed
}

// mountClaiming builds an app with the given claims. With no runtime installed,
// a declared address answers 503 rather than reaching one — which is what lets
// this file read "declared" off the answer without standing a runtime up.
func mountClaiming(t *testing.T, opts ...o11y.Option) *zip.App {
	t.Helper()
	app := zip.New(zip.Config{DisableStartupMessage: true})
	if err := o11y.Mount(app, opts...); err != nil {
		t.Fatalf("Mount: %v", err)
	}
	o11y.SetRuntime(nil)
	return app
}

// The whole table is declared when nothing is claimed. This is the control: it
// is what makes the absences in the next test attributable to the claim rather
// than to the address never having been there.
func TestUnclaimedMountDeclaresEverything(t *testing.T) {
	app := mountClaiming(t)
	for _, a := range []struct{ method, path string }{
		{http.MethodGet, "/v1/o11y/logs"},
		{http.MethodGet, "/v1/o11y/metrics"},
		{http.MethodPost, "/v1/o11y/query_range"},
		{http.MethodGet, "/v1/o11y/logs/livetail"}, // a hatch
	} {
		if !declaredAt(t, app, a.method, a.path) {
			t.Errorf("%s %s is not declared by an unclaimed Mount", a.method, a.path)
		}
	}
}

// A claimed address is not declared, and its NEIGHBOURS are untouched. The
// neighbours are the point: /metrics and /metrics/stats differ by one segment,
// and a seam that matched by prefix would take the second one with the first.
func TestClaimedAddressIsNotDeclared(t *testing.T) {
	app := mountClaiming(t, o11y.Claimed(
		"GET /v1/o11y/metrics",
		"POST /v1/o11y/query_range",
	))

	for _, a := range []struct{ method, path string }{
		{http.MethodGet, "/v1/o11y/metrics"},
		{http.MethodPost, "/v1/o11y/query_range"},
	} {
		if declaredAt(t, app, a.method, a.path) {
			t.Errorf("%s %s was claimed by the host but is still declared", a.method, a.path)
		}
	}

	// Everything the claim did NOT name is still there: the sibling under the
	// claimed path, the same path under a different METHOD, and an unrelated op.
	for _, a := range []struct{ method, path string }{
		{http.MethodPost, "/v1/o11y/metrics/stats"},      // sibling below a claimed path
		{http.MethodGet, "/v1/o11y/query_range"},         // same path, method not claimed
		{http.MethodPost, "/v1/o11y/query_range/format"}, // sibling below a claimed path
		{http.MethodGet, "/v1/o11y/logs"},                // unrelated op
	} {
		if !declaredAt(t, app, a.method, a.path) {
			t.Errorf("%s %s was NOT claimed but the claim took it anyway", a.method, a.path)
		}
	}
}

// A hatch is an address too, so a host can take one. The hatches are raw routes
// rather than typed ops, and they went through a different verb — this is what
// keeps the seam from being true of 353 addresses and false of eleven.
func TestClaimedHatchIsNotDeclared(t *testing.T) {
	app := mountClaiming(t, o11y.Claimed("GET /v1/o11y/logs/livetail"))
	if declaredAt(t, app, http.MethodGet, "/v1/o11y/logs/livetail") {
		t.Error("GET /v1/o11y/logs/livetail was claimed but is still declared")
	}
	if !declaredAt(t, app, http.MethodGet, "/v1/o11y/query_progress") {
		t.Error("an unclaimed hatch stopped being declared")
	}
}

// A claim for an address this table does not declare is INERT, not an error. The
// host is describing what it serves; a table that stopped declaring an address
// must not turn into a boot failure in every host that had claimed it.
func TestClaimForAnUnknownAddressIsInert(t *testing.T) {
	app := mountClaiming(t, o11y.Claimed("GET /v1/o11y/no-such-route", "DELETE /v1/o11y/logs"))
	if !declaredAt(t, app, http.MethodGet, "/v1/o11y/logs") {
		t.Error("a claim on a method this table does not declare removed the one it does")
	}
}

// The claim set does not outlive its Mount. It is read by the declaration verbs
// during the call, so a second Mount with no options must see the whole table
// again — otherwise one host's claim would leak into the next app in the same
// process, which is exactly what a test binary does.
func TestClaimDoesNotLeakToTheNextMount(t *testing.T) {
	_ = mountClaiming(t, o11y.Claimed("GET /v1/o11y/metrics"))
	next := mountClaiming(t)
	if !declaredAt(t, next, http.MethodGet, "/v1/o11y/metrics") {
		t.Error("the previous Mount's claim leaked into a later, unclaimed Mount")
	}
}
