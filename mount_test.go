package o11y_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hanzoai/o11y"
	"github.com/zap-proto/zip"
)

// Mount is total: it must not fail, and it must leave every route the runtime
// serves reachable. The counting proof lives in routes_test.go — this file holds
// what is true of the SEAM itself.

// Mount takes the router and NOTHING else. The signature is the assertion — this
// test only pins that the table is total on a bare app, with no host, no
// dependency struct and no runtime behind it. A table that needed anything from
// its host could not be mounted by o11y's own binary, and for one logger field it
// was not.
func TestMountTakesOnlyTheRouter(t *testing.T) {
	app := zip.New(zip.Config{DisableStartupMessage: true})
	if err := o11y.Mount(app); err != nil {
		t.Fatalf("Mount: %v", err)
	}
}

// A NAMED route with no runtime behind it answers 503, not 404: "the runtime is
// not up yet" is a different fact from "there is no such route", and a caller —
// or a load balancer — needs to be able to tell them apart.
//
// This test used to ask for /v1/o11y/anything and expect 503, which was only
// true because a catch-all answered every path under the prefix. That is the
// assertion that let three unmounted slices pass CI: a wildcard makes an
// unconverted route and a missing one indistinguishable. It now asks for routes
// that actually exist — one hatch and one typed op.
func TestNamedRouteWithoutRuntimeReturns503(t *testing.T) {
	app := zip.New(zip.Config{DisableStartupMessage: true})
	if err := o11y.Mount(app); err != nil {
		t.Fatalf("Mount: %v", err)
	}
	o11y.SetHandler(nil)

	for _, target := range []string{
		"/v1/o11y/logs/livetail", // a hatch
		"/v1/o11y/logs/fields",   // a typed op
	} {
		resp, err := app.Test(httptest.NewRequest(http.MethodGet, target, nil))
		if err != nil {
			t.Fatalf("Test: %v", err)
		}
		if resp.StatusCode != http.StatusServiceUnavailable {
			t.Fatalf("%s status=%d want 503", target, resp.StatusCode)
		}
	}
}

// THE THREE ADDRESSES THIS TABLE DOES NOT CLAIM, pinned as an absence.
//
// GET /v1/o11y/logs, GET /v1/o11y/metrics and POST /v1/o11y/query_range belong to
// the composing HOST. The host declares a tenant-scoped handler at each — it
// resolves the caller's org and pins the read to it — and the relay ops that used
// to sit here did not: they handed the call on with no org attached. A document
// holds one operation per method+path, so both cannot claim the address, and the
// one that must win is the one that keeps a customer out of another customer's
// data.
//
// This is a REGRESSION GUARD, and the regression it guards against is a P0. While
// the composition tolerated a repeated address it resolved one to the first
// declaration and silently dropped the second, so these three were dead code that
// looked alive. The composition now REFUSES the program instead, which means
// re-adding any one of them does not shadow a route — it panics the host at
// Listen and takes the whole subsystem down with a 503. Re-adding is therefore
// not a style question, and this test is what says so before it reaches
// production.
//
// The absence is asserted on the ROUTER, not on a request: a request would prove
// only that nothing answered, which a missing runtime also produces.
func TestHostOwnedAddressesAreNotClaimed(t *testing.T) {
	app := zip.New(zip.Config{DisableStartupMessage: true})
	if err := o11y.Mount(app); err != nil {
		t.Fatalf("Mount: %v", err)
	}

	claimed := map[string]bool{}
	for _, r := range app.Fiber().GetRoutes(true) {
		claimed[r.Method+" "+r.Path] = true
	}
	for _, addr := range []string{
		"GET /v1/o11y/logs",
		"GET /v1/o11y/metrics",
		"POST /v1/o11y/query_range",
	} {
		if claimed[addr] {
			t.Errorf("%s is claimed by this table; it is the host's address, and claiming it "+
				"refuses the composed program at Listen — see the note at the removal site", addr)
		}
	}

	// The METHOD is what yields, not the path. The host declares only POST at
	// query_range, so the legacy GET is still this table's and must stay claimed —
	// otherwise this guard would quietly license deleting it too.
	if !claimed["GET /v1/o11y/query_range"] {
		t.Error("GET /v1/o11y/query_range is not claimed; the host owns only the POST at that path")
	}
}

// Path delegation is proved once, over the whole hatch census, in
// routes_test.go's TestHatchesDelegateVerbatim. It is not repeated here: the
// earlier version of this test sampled three paths by hand and drifted every
// time one of them was converted — including once when it asserted a route
// delegated that had in fact been typed, which ENCODED the dark-slice defect
// instead of catching it. One census, derived from the same list mount.go
// registers, cannot drift that way.
