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

// TestLLMObsRoutes proves the llmobs routes register on the router at the internal
// /api/<resource> paths (the external /v1/o11y/<resource> contract maps onto them via
// the o11y mount) and reflect cleanly through the OpenAPI collector (the same walk the
// spec generator performs), without needing a live instrumentation stack.
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
		"/api/observations": false,
		"/api/traces":       false,
		"/api/sessions":     false,
		"/api/users":        false,
		"/api/scores":       false,
		"/api/score/{id}":   false,
		"/api/annotation":   false,
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

	// Reflect every registered route's OpenAPI definition; a bad request/response
	// DTO would surface here exactly as it would in the spec generator.
	collector := handler.NewOpenAPICollector(openapi3.NewReflector())
	if err := router.Walk(collector.Walker); err != nil {
		t.Fatalf("openapi reflection failed: %v", err)
	}
}
