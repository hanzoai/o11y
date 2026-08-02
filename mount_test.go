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
		"/v1/o11y/logs",          // a typed op
	} {
		resp, err := app.Fiber().Test(httptest.NewRequest(http.MethodGet, target, nil))
		if err != nil {
			t.Fatalf("Test: %v", err)
		}
		if resp.StatusCode != http.StatusServiceUnavailable {
			t.Fatalf("%s status=%d want 503", target, resp.StatusCode)
		}
	}
}

// Path delegation is proved once, over the whole hatch census, in
// routes_test.go's TestHatchesDelegateVerbatim. It is not repeated here: the
// earlier version of this test sampled three paths by hand and drifted every
// time one of them was converted — including once when it asserted a route
// delegated that had in fact been typed, which ENCODED the dark-slice defect
// instead of catching it. One census, derived from the same list mount.go
// registers, cannot drift that way.
