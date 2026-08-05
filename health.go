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
// readiness and health probes under /v1/o11y/* dispatch on the native
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

// mountHealth registers the probe group. The paths are the SAME literals the
// runtime registers, so native dispatch and the fall-through answer the same
// request — one spelling per probe, not two, and each probe's fall-through is now
// the runtime's handler for that probe's OWN address rather than for the whole
// surface.
func mountHealth(app *zip.App) {
	app.Get(o11yRoot+"/livez", livez(o11yRoot+"/livez"))
	app.Get(o11yRoot+"/healthz", probe(o11yRoot+"/healthz", func(h factory.Handler) http.HandlerFunc { return h.Healthz }))
	app.Get(o11yRoot+"/readyz", probe(o11yRoot+"/readyz", func(h factory.Handler) http.HandlerFunc { return h.Readyz }))
}

// The fall-through is the runtime's handler for the probe's own address, reached
// through the same hatch every un-typed route uses (claim.go).
//
// It used to be c.Next(), which worked only because a /v1/o11y/* catch-all was
// registered after the probes and caught whatever they declined. Every route is
// named now and the catch-all is gone, so c.Next() would fall through to a 404
// — the probes would start FAILING the moment the deployment had not called
// SetHealth yet, which is exactly when a liveness probe matters most. Naming the
// destination is both the fix and the honest statement of what "fall through"
// always meant here.

// livez reports process liveness. factory.Handler.Livez renders an empty success
// envelope with 200 unconditionally, so it is rendered natively here through the
// shared render types rather than bridged. Falls through to the runtime when no
// handler is set.
func livez(path string) zip.Handler {
	delegate := hatch(http.MethodGet, path)
	return func(c *zip.Ctx) error {
		if getHealth() == nil {
			return delegate(c)
		}
		return c.JSON(http.StatusOK, render.SuccessResponse{Status: render.StatusSuccess.String()})
	}
}

// probe dispatches a stateful health check (healthz, readyz) through the runtime
// handler selected by sel. The check reads the service registry, so its body
// stays in factory.Handler (one home) and is reached over the net/http bridge;
// the routing is native. Falls through to the runtime when no handler is set.
func probe(path string, sel func(factory.Handler) http.HandlerFunc) zip.Handler {
	delegate := hatch(http.MethodGet, path)
	return func(c *zip.Ctx) error {
		h := getHealth()
		if h == nil {
			return delegate(c)
		}
		return zip.AdaptNetHTTP(sel(h))(c)
	}
}
