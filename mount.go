package o11y

import (
	"net/http"
	"sync"

	"github.com/hanzoai/cloud"
	luxlog "github.com/luxfi/log"
	"github.com/zap-proto/zip"
)

// Mount registers Hanzo o11y's HTTP surface under /v1/o11y per HIP-0106.
//
// The o11y runtime (metrics, traces, logs, dashboards, alerts) is heavy:
// telemetry stores, rule manager, websocket attachments, opamp server.
// The standalone cmd/server boot path constructs it all. To keep the
// route layer composable with the unified cloud binary, Mount delegates
// to a handler registered by the runtime via SetHandler.
//
// Routing model:
//
//   - Standalone: cmd/server/server.go constructs *Server, calls
//     o11y.SetHandler(server.PublicHandler()), then cloud.MountAll wires it.
//   - Cloud binary: same SetHandler call, executed from the cloud bootstrapper
//     after o11y.New + app.NewServer.
//   - Until a handler is registered, the routes 503 with a clear error.
//
// All traffic under /v1/o11y is delegated to the registered http.Handler via
// zip.AdaptNetHTTP, PATH UNTOUCHED. There is no rewrite seam: every route is
// registered at its full public path /v1/o11y/<resource>, so the route literal
// IS the contract — one spelling, nothing to translate, nothing to drift.
func Mount(app *zip.App, deps cloud.Deps) error {
	log := deps.Logger
	if log == nil {
		log = luxlog.New("module", "o11y")
	}
	log.Info("o11y: mounting routes", "prefix", "/v1/o11y")

	// Native probe group, registered ahead of the delegation wildcard so Fiber's
	// in-order match serves it off the mux tree (see health.go).
	mountHealth(app)
	// The TYPED telemetry ops of the error-tracking face — the five reads that
	// now carry a named input, a named output and their prose into the document
	// instead of hiding behind a wildcard. They answer on the runtime, so the
	// wire is unchanged; see telemetry.go.
	mountTelemetry(app)
	// The TYPED infra ops — hosts, processes, the Kubernetes fleet and the
	// infra_monitoring rollups; forty-four reads that now carry named inputs,
	// named outputs and their prose into the document. They answer on the
	// runtime, so the wire is unchanged; see infra.go.
	mountInfra(app)
	// The TYPED logs ops — the record read, the field catalog and its tuning
	// write, the aggregate read, pipelines and path promotion; live tail stays
	// on the wildcard because it is a stream. Same construction: they answer
	// on the runtime, so the wire is unchanged; see logs.go.
	mountLogs(app)
	// The TYPED identity ops — users, invites, passwords, roles-on-users,
	// sessions, auth domains, my-org, preferences and quick filters. Same
	// construction: named In, named Out, answered on the runtime, wire
	// unchanged; see identity.go. The three /complete/* sign-in callbacks
	// stay on the wildcard below — they answer with redirects, not JSON.
	mountIdentity(app)
	// The TYPED query-core ops — the v5 composite query engine (query_range, its
	// dry-run preview, variable substitution), the legacy metrics range read and
	// the builder-format echo, attribute autocomplete, the field catalog and the
	// saved explorer views. Same construction: named In, named Out, answered on
	// the runtime, wire unchanged; see querycore.go. Query progress (long-poll
	// and websocket) and raw-data export stay on the wildcard below — they are
	// streams, not JSON answers.
	mountQueryCore(app)
	app.All("/v1/o11y/*", zip.AdaptNetHTTP(handlerAdapter{}))
	return nil
}

// handlerAdapter forwards each request to the registered runtime handler
// or returns 503 if none is set yet. It does NOT touch r.URL: the runtime's
// routes are registered at their full public /v1/o11y/<resource> paths, so a
// rewrite here could only ever move a request OFF its route. The previous
// rewrite onto an /api/ namespace outlived that namespace and 404'd everything.
type handlerAdapter struct{}

func (handlerAdapter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h := getHandler()
	if h == nil {
		http.Error(w, "o11y runtime not initialized", http.StatusServiceUnavailable)
		return
	}
	h.ServeHTTP(w, r)
}

var (
	hmu        sync.RWMutex
	registered http.Handler
)

// SetHandler registers the o11y runtime's public HTTP handler. The
// standalone server calls this after app.NewServer; the unified cloud
// binary calls it after constructing the same runtime in-process.
// Safe for concurrent use; pass nil to unset.
func SetHandler(h http.Handler) {
	hmu.Lock()
	registered = h
	hmu.Unlock()
}

func getHandler() http.Handler {
	hmu.RLock()
	h := registered
	hmu.RUnlock()
	return h
}
