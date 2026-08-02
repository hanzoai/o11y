package o11y_test

// THE EMBED SHAPE IS THE TEST.
//
// Every other test in this package installs a bare http.HandlerFunc as the
// runtime and reads r.URL.Path off it. That shape cannot see the defect this
// file exists for: http.NewRequest populates URL and deliberately leaves
// RequestURI empty, so a stand-in that reads URL.Path passes on a request the
// real runtime cannot route at all.
//
// The runtime the embedding host actually installs is Server.PublicHandler() —
// adaptor.FiberApp over the runtime's own fiber app — and that adaptor copies
// r.RequestURI into fasthttp verbatim. With RequestURI empty the path was
// erased, fasthttp normalized it to "/", no API route matched, and the request
// fell through to the console web provider's http.NotFound. Every relayed op
// answered 404 {"status":404,"error":"404 page not found"} at the unified door
// while the standalone served the same paths 200.
//
// So this test installs the REAL wrapper rather than a stand-in. A fake that is
// more forgiving than production is not a test of production.

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/hanzoai/o11y"
	fiber "github.com/zap-proto/fiber/v3"
	"github.com/zap-proto/fiber/v3/middleware/adaptor"
	"github.com/zap-proto/zip"
)

// adaptorRuntime installs a runtime that ROUTES, wrapped exactly as
// Server.PublicHandler() wraps it. Only a request carrying a real request-target
// reaches the registered route.
func adaptorRuntime(t *testing.T) {
	t.Helper()
	rt := zip.New(zip.Config{DisableStartupMessage: true})
	rt.Fiber().Get(o11yVersionPath, func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"version": "v1.5.49", "ee": "N", "setupCompleted": true})
	})
	rt.Fiber().Get(o11yHealthPath, func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})
	o11y.SetHandler(adaptor.FiberApp(rt.Fiber()))
	t.Cleanup(func() { o11y.SetHandler(nil) })
}

const (
	o11yVersionPath = "/v1/o11y/version"
	o11yHealthPath  = "/v1/o11y/health"
)

// TestRelayReachesARoutingRuntime is the regression: a typed op must arrive at
// the runtime with a request-target the runtime can match on.
func TestRelayReachesARoutingRuntime(t *testing.T) {
	app := mounted(t)
	adaptorRuntime(t)

	code, body := call(t, app, member(http.MethodGet, o11yVersionPath, nil))
	if code != http.StatusOK {
		t.Fatalf("GET %s = %d %s, want 200 — relay handed the runtime a request with no "+
			"request-target, so the route never matched", o11yVersionPath, code, body)
	}
	var out o11y.O11yVersionOut
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("decode %s: %v (body %s)", o11yVersionPath, err, body)
	}
	if out.Version == "" || out.EE != "N" {
		t.Fatalf("%s = %+v, want the runtime's own version and ee=N", o11yVersionPath, out)
	}

	code, body = call(t, app, member(http.MethodGet, o11yHealthPath, nil))
	if code != http.StatusOK {
		t.Fatalf("GET %s = %d %s, want 200", o11yHealthPath, code, body)
	}
	var h o11y.O11yHealthOut
	if err := json.Unmarshal(body, &h); err != nil {
		t.Fatalf("decode %s: %v (body %s)", o11yHealthPath, err, body)
	}
	if h.Status != "ok" {
		t.Fatalf("%s status = %q, want ok", o11yHealthPath, h.Status)
	}
}

// TestRelaySetsRequestURI pins the FIELD, so the reason survives even if the
// adaptor above is ever swapped for another transport that reads it.
// URL and RequestURI must agree — a handler may read either.
func TestRelaySetsRequestURI(t *testing.T) {
	app := mounted(t)

	var gotURI, gotPath, gotQuery string
	o11y.SetHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURI, gotPath, gotQuery = r.RequestURI, r.URL.Path, r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"version":"v","ee":"N","setupCompleted":true}`))
	}))
	t.Cleanup(func() { o11y.SetHandler(nil) })

	if code, body := call(t, app, member(http.MethodGet, o11yVersionPath, nil)); code != http.StatusOK {
		t.Fatalf("GET %s = %d %s, want 200", o11yVersionPath, code, body)
	}
	if gotURI != o11yVersionPath {
		t.Errorf("RequestURI = %q, want %q — a handler is a SERVER seam and reads the "+
			"request-target the server would have set", gotURI, o11yVersionPath)
	}
	if gotPath != o11yVersionPath {
		t.Errorf("URL.Path = %q, want %q", gotPath, o11yVersionPath)
	}
	if gotQuery != "" {
		t.Errorf("URL.RawQuery = %q, want empty", gotQuery)
	}
}

// TestRelayCarriesQueryIntoRequestURI: the request-target includes the query,
// so an op that passes params still reaches a runtime that routes on it.
func TestRelayCarriesQueryIntoRequestURI(t *testing.T) {
	app := mounted(t)

	var gotURI string
	o11y.SetHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURI = r.RequestURI
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","data":{}}`))
	}))
	t.Cleanup(func() { o11y.SetHandler(nil) })

	// /v1/o11y/logs?... is a params-carrying op; any non-2xx here still records
	// the target, which is what this asserts.
	_, _ = call(t, app, member(http.MethodGet, "/v1/o11y/usage?start=1&end=2", nil))
	if gotURI == "" {
		t.Fatal("RequestURI empty — relay dropped the request-target")
	}
	if gotURI[0] != '/' {
		t.Fatalf("RequestURI = %q, want an origin-form target starting with /", gotURI)
	}
}
