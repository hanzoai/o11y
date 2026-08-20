package o11y_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hanzoai/o11y"
	"github.com/zap-proto/zip"
)

// THE CENSUS. This file is the arithmetic that the wildcard used to hide.
//
// While Mount ended in app.All("/v1/o11y/*", …) there was no way to tell a
// converted route from an unconverted one from a MISSING one: the catch-all
// answered all three the same. That is exactly how 83 typed ops shipped in
// files nothing called — the package built, the suite passed, and every one of
// those routes still fell through the wildcard. These tests are the replacement
// for that catch-all's silence.

// registered is every non-HEAD, non-OPTIONS route the mount actually installed.
func registered(t *testing.T, app *zip.App) map[string]bool {
	t.Helper()
	got := map[string]bool{}
	for _, r := range app.Fiber().GetRoutes(true) {
		if r.Method == http.MethodHead || r.Method == http.MethodOptions {
			continue
		}
		got[r.Method+" "+r.Path] = true
	}
	return got
}

// assertRoutes proves a slice registered EXACTLY its own routes under prefix —
// no more, no fewer, at the same methods. Each slice counts only its own doors,
// so a slice that stops being mounted fails here rather than being quietly
// answered by something else.
func assertRoutes(t *testing.T, want map[string]bool, prefix string) {
	t.Helper()
	got := map[string]bool{}
	for route := range registered(t, mounted(t)) {
		_, path, _ := strings.Cut(route, " ")
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			got[route] = true
		}
	}
	for route := range want {
		if !got[route] {
			t.Errorf("%s is not registered", route)
		}
	}
	for route := range got {
		if !want[route] {
			t.Errorf("%s is registered but not in the census", route)
		}
	}
}

// THE WHOLE SURFACE, counted. 367 is the number of method+path pairs the o11y
// runtime registers on its gorilla/mux tree (pkg/query-service/app/routes_*.go
// and pkg/apiserver/o11yapiserver/*.go). Mount registers the same 367 — 353 as
// typed ops that carry a named In, a named Out and their prose into the
// document, 11 as named escape hatches, 3 as native service probes.
//
// If this number moves, one of two things happened and both need a human: a
// route was added to the runtime and not named here (it would 404 in the
// composed binary), or a route was named here that the runtime does not serve
// (it would 404 at the runtime). Neither is visible without the count.
func TestEveryRouteIsNamedAndCounted(t *testing.T) {
	app := mounted(t)
	all := registered(t, app)

	const (
		wantTyped   = 353 // typed ops: in the OpenAPI document and the MCP tool list
		wantHatches = 11  // mount.go mountHatches, each with its reason
		wantProbes  = 3   // health.go livez/healthz/readyz, native with fall-through
	)
	if len(all) != wantTyped+wantHatches+wantProbes {
		t.Fatalf("mount registered %d routes, want %d", len(all), wantTyped+wantHatches+wantProbes)
	}

	// The typed ops are exactly the operations zip published. Counting them off
	// the document rather than off the router is what makes this a payoff
	// measurement and not a tautology: an op that reaches the router but not the
	// document is the defect this whole conversion exists to remove.
	spec := app.OpenAPISpec()
	if got := len(app.MCPTools()); got != wantTyped {
		t.Fatalf("MCP tools = %d, want %d", got, wantTyped)
	}
	paths, ok := spec["paths"].(map[string]map[string]any)
	if !ok {
		t.Fatalf("OpenAPI paths has type %T", spec["paths"])
	}
	ops := 0
	for path, item := range paths {
		if !strings.HasPrefix(path, "/v1/o11y") && !strings.HasPrefix(path, "/v1/sentinel") {
			t.Errorf("OpenAPI published %s, which is not o11y's surface", path)
		}
		for method := range item {
			switch method {
			case "get", "post", "put", "patch", "delete":
				ops++
			}
		}
	}
	if ops != wantTyped {
		t.Fatalf("OpenAPI operations = %d, want %d", ops, wantTyped)
	}
}

// No route may end in a wildcard. This is the invariant the whole change buys:
// a NEW un-typed route is a 404 someone has to come and justify, not a silent
// fall-through into the runtime.
func TestNoCatchAllRemains(t *testing.T) {
	for route := range registered(t, mounted(t)) {
		if strings.HasSuffix(route, "*") || strings.HasSuffix(route, "/+") {
			t.Errorf("%s is a catch-all", route)
		}
	}
}

// A path under /v1/o11y that nothing named is a 404 — proof the catch-all is
// gone. Under the wildcard this same request reached the runtime, which is why
// an unmounted slice could go unnoticed.
func TestUnnamedPathIsNotServed(t *testing.T) {
	app := mounted(t)
	served := false
	setRuntime(t, func(w http.ResponseWriter, r *http.Request) {
		served = true
		w.WriteHeader(http.StatusOK)
	})

	status, _ := call(t, app, member(http.MethodGet, "/v1/o11y/no-such-thing", nil))
	if status != http.StatusNotFound {
		t.Fatalf("status=%d want 404", status)
	}
	if served {
		t.Fatal("an unnamed path still reached the runtime")
	}
}

// setRuntime installs a stand-in runtime for the duration of the test.
func setRuntime(t *testing.T, fn http.HandlerFunc) {
	t.Helper()
	o11y.SetRuntime(o11y.Whole(fn))
	t.Cleanup(func() { o11y.SetRuntime(nil) })
}

// THE ELEVEN. Each hatch is registered at its exact method and path, and each
// delegates with the path UNTOUCHED — a rewrite at this seam can only ever move
// a request off its own route, and a mangler is invisible to the compiler, so
// this is the only thing that catches one coming back.
func TestHatchesDelegateVerbatim(t *testing.T) {
	hatches := []struct{ method, path string }{
		{http.MethodGet, "/v1/o11y/logs/livetail"},
		{http.MethodGet, "/v1/o11y/query_progress"},
		{http.MethodGet, "/ws/query_progress"},
		{http.MethodPost, "/v1/o11y/export_raw_data"},

		{http.MethodGet, "/v1/o11y/complete/google"},
		{http.MethodGet, "/v1/o11y/complete/oidc"},
		{http.MethodPost, "/v1/o11y/complete/saml"},

		{http.MethodPost, "/v1/event/6ba7b810-9dad-11d1-80b4-00c04fd430c8/envelope/"},
		{http.MethodPost, "/v1/event/6ba7b810-9dad-11d1-80b4-00c04fd430c8/store/"},
		{http.MethodPost, "/v1/o11y/api/6ba7b810-9dad-11d1-80b4-00c04fd430c8/envelope/"},
		{http.MethodPost, "/v1/o11y/api/6ba7b810-9dad-11d1-80b4-00c04fd430c8/store/"},
	}
	if len(hatches) != 11 {
		t.Fatalf("the census itself is wrong: %d", len(hatches))
	}

	app := mounted(t)
	var saw string
	setRuntime(t, func(w http.ResponseWriter, r *http.Request) {
		saw = r.URL.Path
		w.WriteHeader(http.StatusOK)
	})

	for _, h := range hatches {
		t.Run(h.method+" "+h.path, func(t *testing.T) {
			saw = ""
			status, body := call(t, app, member(h.method, h.path, strings.NewReader("{}")))
			if status != http.StatusOK {
				t.Fatalf("status=%d body=%s", status, body)
			}
			if saw != h.path {
				t.Fatalf("runtime received %q, want %q verbatim", saw, h.path)
			}
		})
	}
}

// A hatch answers 503 rather than 404 while no runtime is registered — the same
// honest "not initialized yet" the wildcard gave, kept now that each hatch is
// its own route.
func TestHatchWithoutRuntimeIs503(t *testing.T) {
	app := mounted(t)
	o11y.SetRuntime(nil)
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/v1/o11y/logs/livetail", nil))
	if err != nil {
		t.Fatalf("Test: %v", err)
	}
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status=%d want 503", resp.StatusCode)
	}
}

// TestUnderIsStillLoadBearing measures why relay.go's `under` cannot be a
// Group, instead of asserting it in prose.
//
// zip v1.24.2 stopped composition from renaming a DECLARED operation id, which
// removed half the reason `under` exists. The other half is still here: an op
// with no declared id takes zip's shape-derived one, and THAT is qualified by
// the prefix of the occurrence it answers under. Registering this package's ops
// through app.Group(o11yRoot) would therefore publish every undeclared one as
// "v1.o11y.<derived>" — the same rename that cost 353 ids, applied to however
// many of them this test finds.
//
// The number is the point. When it reaches zero — every op declares its id —
// `under` can be deleted and the registration can be a plain Group, and this is
// the test that will say so.
func TestUnderIsStillLoadBearing(t *testing.T) {
	app := mounted(t)
	if err := app.Build(); err != nil {
		t.Fatalf("Build: %v", err)
	}

	declared, undeclared := 0, 0
	for _, op := range app.Registry() {
		if op.OperationID == "" {
			undeclared++
			continue
		}
		declared++
	}
	if declared+undeclared != 353 {
		t.Fatalf("registry holds %d ops, want the 353 of TestEveryRouteIsNamedAndCounted", declared+undeclared)
	}
	if undeclared == 0 {
		t.Fatalf("every one of the %d ops now declares its id — a declared id survives "+
			"composition verbatim, so `under` has nothing left to protect: replace it with "+
			"app.Group(o11yRoot) and delete it", declared)
	}
	t.Logf("%d ops declare an id and survive any prefix; %d do not and would be "+
		"renamed by a Group — `under` stays", declared, undeclared)

	// The property itself, on the real mount: nothing this service publishes
	// carries the occurrence qualification, declared or not.
	for _, tool := range app.MCPTools() {
		name, _ := tool["name"].(string)
		if strings.HasPrefix(name, "v1.") {
			t.Errorf("published name %q is prefix-qualified — the registration reached a Group", name)
		}
	}
}
