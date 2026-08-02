package routing_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hanzoai/o11y/pkg/http/handler"
	"github.com/hanzoai/o11y/pkg/http/routing"
	"github.com/hanzoai/o11y/pkg/types/coretypes"
	"github.com/zap-proto/fiber/v3/middleware/adaptor"
	"github.com/zap-proto/zip"
)

// THE CENSUS. Every registration must reach the router, and every route on the
// router must have come from a registration. This repo has already shipped the
// other outcome once: three slices of typed ops landed with their files present
// and nothing calling them, so 83 routes existed in the source and reached no
// router at all — the package built and the whole suite passed while every one of
// them stayed dark. An uncalled func is legal Go, so arithmetic is the only thing
// that catches it. That is what the Table is for and this is the test of it.
func TestTableAndRouterAgree(t *testing.T) {
	app := zip.New(zip.Config{DisableStartupMessage: true})
	r := routing.New(app.Group(""), nil)

	r.Get("/v1/o11y/rules", nothing())
	r.Post("/v1/o11y/rules", nothing())
	r.Get("/v1/o11y/rules/{id}", nothing())
	r.Put("/v1/o11y/rules/{id}", nothing())
	r.Patch("/v1/o11y/rules/{id}", nothing())
	r.Delete("/v1/o11y/rules/{id}", nothing())
	pods := r.Group("/v1/o11y/pods")
	pods.Get("/attribute_keys", nothing())
	pods.Post("/list", nothing())

	want := map[string]bool{
		"GET /v1/o11y/rules":               true,
		"POST /v1/o11y/rules":              true,
		"GET /v1/o11y/rules/{id}":          true,
		"PUT /v1/o11y/rules/{id}":          true,
		"PATCH /v1/o11y/rules/{id}":        true,
		"DELETE /v1/o11y/rules/{id}":       true,
		"GET /v1/o11y/pods/attribute_keys": true,
		"POST /v1/o11y/pods/list":          true,
	}

	got := map[string]bool{}
	for _, route := range r.Table().Routes() {
		got[route.Method+" "+route.Path] = true
	}
	for route := range want {
		if !got[route] {
			t.Errorf("%s is not in the table", route)
		}
	}
	for route := range got {
		if !want[route] {
			t.Errorf("%s is in the table but not in the census", route)
		}
	}

	// And the router itself holds exactly those, in the router's own spelling.
	onRouter := map[string]bool{}
	for _, route := range app.Fiber().GetRoutes(true) {
		if route.Method == http.MethodHead || route.Method == http.MethodOptions {
			continue
		}
		onRouter[route.Method+" "+route.Path] = true
	}
	if len(onRouter) != len(want) {
		t.Fatalf("router holds %d routes, the table holds %d", len(onRouter), len(want))
	}
	for route := range want {
		method, path, _ := strings.Cut(route, " ")
		spelled := method + " " + strings.ReplaceAll(strings.ReplaceAll(path, "{", ":"), "}", "")
		if !onRouter[spelled] {
			t.Errorf("%s is in the table but %s is not on the router", route, spelled)
		}
	}
}

// A route registered twice is a mistake in the source, and the tree this
// replaces answered it by silently letting the first one win.
func TestDuplicateRegistrationPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("a second registration at the same door did not panic")
		}
	}()
	app := zip.New(zip.Config{DisableStartupMessage: true})
	r := routing.New(app.Group(""), nil)
	r.Get("/v1/o11y/rules", nothing())
	r.Get("/v1/o11y/rules", nothing())
}

// The handler reads the segment the ROUTER matched, through the one seam, and
// the template it reads back is the registered literal rather than the request.
func TestRouteValuesReachTheHandler(t *testing.T) {
	var id, template string
	served := routing.Serve(http.MethodGet, "/v1/o11y/rules/{id}", http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
		id, template = coretypes.Param(req, "id"), coretypes.RoutePath(req)
	}))

	rec := httptest.NewRecorder()
	served.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/o11y/rules/7?id=99", http.NoBody))

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d — the route did not match, so nothing was proved", rec.Code)
	}
	if id != "7" {
		t.Errorf("Param(id)=%q want 7 (a query value of the same name must not answer)", id)
	}
	if template != "/v1/o11y/rules/{id}" {
		t.Errorf("RoutePath=%q want the registered template", template)
	}
}

// A constrained segment only matches values it accepts: the two Sentry ingest
// routes take a UUID project and nothing else, so a resource word can never be
// swallowed by the wildcard that follows the static routes.
func TestConstrainedSegmentRefusesAValueItDoesNotAccept(t *testing.T) {
	served := routing.Serve(http.MethodPost, "/v1/sentry/{project:guid}/envelope/", nothing())

	for path, want := range map[string]int{
		"/v1/sentry/6ba7b810-9dad-11d1-80b4-00c04fd430c8/envelope/": http.StatusNoContent,
		"/v1/sentry/projects/envelope/":                             http.StatusNotFound,
	} {
		rec := httptest.NewRecorder()
		served.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, http.NoBody))
		if rec.Code != want {
			t.Errorf("POST %s = %d, want %d", path, rec.Code, want)
		}
	}
}

// The registered path keeps its public spelling in the table even when the route
// carries a constraint the router needs — the document and the census name what a
// client writes, not what the matcher reads.
func TestTableRecordsThePublicSpelling(t *testing.T) {
	app := zip.New(zip.Config{DisableStartupMessage: true})
	r := routing.New(app.Group(""), nil)
	r.Post("/v1/sentry/{project:guid}/envelope/", nothing())

	if got := r.Table().Routes()[0].Path; got != "/v1/sentry/{project}/envelope/" {
		t.Fatalf("table records %q, want the constraint dropped", got)
	}
}

// The chain wraps every leaf, and it is handed the route's OWN declared
// resources — the fact the router knows and the middleware used to re-derive by
// asking the router what it had just matched.
func TestChainWrapsEachLeafWithItsOwnDefs(t *testing.T) {
	var seen []int
	chain := func(defs []handler.ResourceDef) func(http.Handler) http.Handler {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				seen = append(seen, len(defs))
				next.ServeHTTP(w, req)
			})
		}
	}

	app := zip.New(zip.Config{DisableStartupMessage: true})
	r := routing.New(app.Group(""), chain)
	r.Get("/v1/o11y/bare", nothing())
	r.Get("/v1/o11y/declared", handler.New(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}, handler.OpenAPIDef{ID: "Declared"}, handler.WithResourceDefs(handler.BasicResourceDef{})))

	served := adaptor.FiberApp(app.Fiber())
	for _, path := range []string{"/v1/o11y/bare", "/v1/o11y/declared"} {
		served.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, path, http.NoBody))
	}

	if len(seen) != 2 || seen[0] != 0 || seen[1] != 1 {
		t.Fatalf("chain saw defs %v, want [0 1]", seen)
	}
}

func nothing() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
}

// A constraint the router does not know is DROPPED by it, and a dropped
// constraint matches everything — so the wildcard it was guarding starts
// swallowing the static words it sits behind. The router is silent about this;
// the registrar is not.
func TestUnknownConstraintPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("a constraint the router would drop was accepted")
		}
	}()
	app := zip.New(zip.Config{DisableStartupMessage: true})
	routing.New(app.Group(""), nil).
		Post("/v1/sentry/{project:[0-9a-fA-F]{8}}/envelope/", nothing())
}
