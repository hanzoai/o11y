package o11y

import (
	"net/http"
	"sync"

	"github.com/hanzoai/o11y/pkg/factory"
	"github.com/hanzoai/o11y/pkg/http/render"
	"github.com/zap-proto/zip"
)

// The service-health probe group — liveness, readiness, health — dispatches on
// the native zip/fiber router instead of the embedded gorilla/mux tree. It is
// the first route group moved off mux at the Hanzo-owned mount seam: the three
// probes are one cohesive group backed by the ONE runtime health handler
// (factory.Handler), so the health computation keeps its single home while the
// path→handler dispatch becomes native. Every other /v1/o11y route still reaches
// the mux tree through the delegation wildcard (see Mount); the staged migration
// moves further groups here.
//
// Routing model: mountHealth registers the probes AHEAD of the /v1/o11y/*
// wildcard so Fiber's in-order match gives them precedence. Until the runtime
// registers its handler via SetHealth, each probe falls through (Next) to the
// wildcard, so behavior is identical to the delegated path — the native
// dispatch activates the moment SetHealth is wired, with no route change.

var (
	healthMu     sync.RWMutex
	healthSource factory.Handler
)

// SetHealth registers the runtime's service-health handler so the liveness,
// readiness and health probes under /v1/o11y/api/v2/* dispatch on the native
// router. The embedding host calls it after constructing the runtime, passing
// factory.NewHandler(runtime.Registry). Safe for concurrent use; pass nil to
// unset (the probes then fall through to the delegated runtime handler).
func SetHealth(h factory.Handler) {
	healthMu.Lock()
	healthSource = h
	healthMu.Unlock()
}

func getHealth() factory.Handler {
	healthMu.RLock()
	h := healthSource
	healthMu.RUnlock()
	return h
}

// mountHealth registers the probe group on the native router, ahead of the
// /v1/o11y/* delegation wildcard. The external paths mirror the internal
// /api/v2/* routes the runtime already serves, so the public contract is
// unchanged — only the dispatch moves off mux.
func mountHealth(app *zip.App) {
	app.Get("/v1/o11y/api/v2/livez", livez)
	app.Get("/v1/o11y/api/v2/healthz", probe(func(h factory.Handler) http.HandlerFunc { return h.Healthz }))
	app.Get("/v1/o11y/api/v2/readyz", probe(func(h factory.Handler) http.HandlerFunc { return h.Readyz }))
}

// livez reports process liveness. factory.Handler.Livez renders an empty success
// envelope with 200 unconditionally, so it is rendered natively here through the
// shared render types rather than bridged. Falls through when no handler is set.
func livez(c *zip.Ctx) error {
	if getHealth() == nil {
		return c.Next()
	}
	return c.JSON(http.StatusOK, render.SuccessResponse{Status: render.StatusSuccess.String()})
}

// probe dispatches a stateful health check (healthz, readyz) through the runtime
// handler selected by sel. The check reads the service registry, so its body
// stays in factory.Handler (one home) and is reached over the net/http bridge;
// the routing is native. Falls through when no handler is set.
func probe(sel func(factory.Handler) http.HandlerFunc) zip.Handler {
	return func(c *zip.Ctx) error {
		h := getHealth()
		if h == nil {
			return c.Next()
		}
		return zip.AdaptNetHTTPFunc(sel(h))(c)
	}
}
