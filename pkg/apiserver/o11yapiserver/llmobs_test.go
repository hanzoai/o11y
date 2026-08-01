package o11yapiserver

import (
	"log/slog"
	"testing"

	"github.com/gorilla/mux"
	"github.com/hanzoai/o11y/pkg/http/handler"
	"github.com/hanzoai/o11y/pkg/http/middleware"
	"github.com/hanzoai/o11y/pkg/modules/llmobs"
	"github.com/swaggest/openapi-go/openapi3"
)

// TestLLMObsRoutes proves the llmobs routes register at their PUBLIC paths —
// /v1/o11y/llm/<resource>, the literal a client sends, with nothing rewriting it
// in between — and reflect cleanly through the OpenAPI collector (the same walk
// the spec generator performs), without needing a live instrumentation stack.
func TestLLMObsRoutes(t *testing.T) {
	p := &provider{
		llmObsHandler:   struct{ llmobs.Handler }{},
		authzMiddleware: middleware.NewAuthZ(slog.Default(), nil, nil),
	}

	router := mux.NewRouter()
	if err := p.addLLMObsRoutes(router); err != nil {
		t.Fatalf("addLLMObsRoutes: %v", err)
	}

	want := map[string]bool{
		"/v1/o11y/llm/observations": false,
		"/v1/o11y/llm/traces":       false,
		"/v1/o11y/llm/sessions":     false,
		"/v1/o11y/llm/users":        false,
		"/v1/o11y/llm/scores":       false,
		"/v1/o11y/llm/score/{id}":   false,
		"/v1/o11y/llm/annotation":   false,
	}
	err := router.Walk(func(route *mux.Route, _ *mux.Router, _ []*mux.Route) error {
		if path, err := route.GetPathTemplate(); err == nil {
			if _, ok := want[path]; ok {
				want[path] = true
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	for path, seen := range want {
		if !seen {
			t.Errorf("route %s not registered", path)
		}
	}

	// Reflect every registered route's OpenAPI definition: a request or response
	// DTO the reflector cannot describe is a DTO no typed op can carry either.
	collector := handler.NewOpenAPICollector(openapi3.NewReflector())
	if err := router.Walk(collector.Walker); err != nil {
		t.Fatalf("openapi reflection failed: %v", err)
	}
}
