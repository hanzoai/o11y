package o11yapiserver

import (
	"log/slog"
	"testing"

	"github.com/hanzoai/o11y/pkg/http/handler"
	"github.com/hanzoai/o11y/pkg/http/middleware"
	"github.com/hanzoai/o11y/pkg/http/routing"
	"github.com/hanzoai/o11y/pkg/modules/llmobs"
	"github.com/swaggest/openapi-go/openapi3"
	"github.com/zap-proto/zip"
)

// TestLLMObsRoutes proves the llmobs routes register at their PUBLIC paths —
// /v1/o11y/llm/<resource>, the literal a client sends, with nothing rewriting it
// in between — and reflect cleanly through the OpenAPI collector, without
// needing a live instrumentation stack.
//
// The census comes off the registrar's own Table rather than off a walk of the
// router, because the Table IS what registration produced: a route that reached
// the router but not the table, or the reverse, is the defect this whole
// conversion exists to make impossible.
func TestLLMObsRoutes(t *testing.T) {
	p := &provider{
		llmObsHandler:   struct{ llmobs.Handler }{},
		authzMiddleware: middleware.NewAuthZ(slog.Default(), nil, nil),
	}

	app := zip.New(zip.Config{DisableStartupMessage: true})
	router := routing.New(app.Group(""), nil)
	p.addLLMObsRoutes(router)

	want := map[string]bool{
		"/v1/o11y/llm/observations": false,
		"/v1/o11y/llm/traces":       false,
		"/v1/o11y/llm/sessions":     false,
		"/v1/o11y/llm/users":        false,
		"/v1/o11y/llm/scores":       false,
		"/v1/o11y/llm/score/{id}":   false,
		"/v1/o11y/llm/annotation":   false,
	}
	for _, route := range router.Table().Routes() {
		if _, ok := want[route.Path]; ok {
			want[route.Path] = true
		}
	}
	for path, seen := range want {
		if !seen {
			t.Errorf("route %s not registered", path)
		}
	}

	// Reflect every registered route's OpenAPI definition: a request or response
	// DTO the reflector cannot describe is a DTO no typed op can carry either.
	collector := handler.NewOpenAPICollector(openapi3.NewReflector())
	for _, route := range router.Table().Routes() {
		if err := collector.Collect(route.Method, route.Path, route.Handler); err != nil {
			t.Fatalf("openapi reflection failed for %s %s: %v", route.Method, route.Path, err)
		}
	}
}
