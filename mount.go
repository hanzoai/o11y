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
// EVERY ROUTE IS NAMED. There is no /v1/o11y/* catch-all any more. The runtime
// registers 367 method+path pairs; 356 of them are typed ops declared in the
// slice files below, and the remaining 11 are registered one by one in
// mountHatches with the reason each cannot be typed written next to it. A
// catch-all hides the difference between "converted" and "not converted" — the
// dark-slice defect this package already shipped once was invisible precisely
// because a wildcard answered for the routes nobody had wired up. A named route
// per hatch makes the escape hatches countable, and makes any NEW un-typed route
// a 404 that someone has to come and justify rather than a silent fall-through.
//
// PATH UNTOUCHED. Delegation never rewrites r.URL: every route is registered at
// its full public path, so the route literal IS the contract — one spelling,
// nothing to translate, nothing to drift.
func Mount(app *zip.App, deps cloud.Deps) error {
	log := deps.Logger
	if log == nil {
		log = luxlog.New("module", "o11y")
	}
	log.Info("o11y: mounting routes", "prefix", o11yRoot)

	// Native probe group, registered ahead of everything else so Fiber's
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
	// a hatch because it is a stream. Same construction: they answer on the
	// runtime, so the wire is unchanged; see logs.go.
	mountLogs(app)
	// The TYPED identity ops — users, invites, passwords, roles-on-users,
	// sessions, auth domains, my-org, preferences and quick filters. Same
	// construction: named In, named Out, answered on the runtime, wire
	// unchanged; see identity.go. The three /complete/* sign-in callbacks stay
	// hatches — they answer with redirects, not JSON.
	mountIdentity(app)
	// The TYPED access-control ops — roles, service accounts, service-account
	// keys and the authorization probe. Same construction: named In, named Out,
	// answered on the runtime, so the wire — including every CheckResources and
	// OpenAccess gate — is unchanged; see access.go.
	mountAccess(app)
	// The TYPED APM ops — the service catalog, the messaging-queue views and
	// the third-party API overview; twenty-one reads that now carry named
	// inputs, named outputs and their prose into the document. Same
	// construction: named In, named Out, answered on the runtime, wire
	// unchanged; see apm.go.
	mountAPM(app)
	// The TYPED dashboard ops — v2 dashboard CRUD, cloning, locking, per-user
	// pinning, the org-shared saved views, public-sharing config and the two
	// anonymous public reads; twenty-two routes that now carry named inputs,
	// named outputs and their prose into the document. Same construction: they
	// answer on the runtime, so the wire is unchanged; see dashboards.go.
	mountDashboards(app)
	// The TYPED LLM-observability ops — the four gen_ai span views
	// (observations, traces, sessions, users), eval scores, human annotations
	// and the LLM pricing rules; fourteen reads and writes that now carry named
	// inputs, named outputs and their prose into the document. Same
	// construction: they answer on the runtime, so the wire is unchanged; see
	// llmobs.go.
	mountLLMObs(app)
	// The TYPED metrics ops — the metrics-explorer reads and the metadata write,
	// and the volume-control (metric reduction) rules; nineteen ops that now
	// carry named inputs, named outputs and their prose into the document. They
	// answer on the runtime, so the wire is unchanged; see metrics.go.
	mountMetrics(app)
	// The TYPED platform ops — instant queries, dashboard variables, product
	// events, usage, the dependency graph, version/health, disks, first-user
	// registration, retention and apdex settings, licenses, global config,
	// feature flags, org stats, filter suggestions, k8s onboarding, metric
	// metadata, span percentiles and query-filter analysis. Same construction:
	// they answer on the runtime through the one shared relay seam, so the wire
	// is unchanged; see platform.go. The three service probes (healthz, readyz,
	// livez) stay on mountHealth above — its fall-through-when-unset is
	// load-bearing.
	mountPlatform(app)
	// The TYPED query-core ops — the v5 composite query engine (query_range, its
	// dry-run preview, variable substitution), the legacy metrics range read and
	// the builder-format echo, attribute autocomplete, the field catalog and the
	// saved explorer views. Same construction: named In, named Out, answered on
	// the runtime, wire unchanged; see querycore.go. Query progress (long-poll
	// and websocket) and raw-data export stay hatches — they are streams, not
	// JSON answers.
	mountQueryCore(app)
	// The last three slices of the first conversion pass. They were DARK: their
	// files landed with the rest of the conversion, but nothing here called them,
	// so 83 typed ops existed in the source and reached no router at all. An
	// uncalled package-level func is legal Go, so the package built and the whole
	// suite passed while every one of those routes still fell through the
	// wildcard — the conversion was true of the source and false of the binary.
	// Route-table arithmetic is what caught it, and it is why the wildcard is
	// gone: with every route named, a slice that is not mounted 404s loudly
	// instead of being quietly answered by a catch-all.
	mountSentryErrors(app)
	mountRulesAlerts(app)
	mountIntegrations(app)
	// The TYPED traces ops — one trace's spans, the trace field catalog and its
	// tuning write, and the three trace-detail views (waterfall, flamegraph,
	// span aggregations); see traces.go.
	mountTraces(app)
	// The TYPED trace-funnel ops — funnel CRUD plus the twelve analytics reads,
	// six over a saved funnel and six over one described inline; see
	// tracefunnels.go.
	mountTraceFunnels(app)
	// The TYPED span-mapper ops — the ingest-time rules that move or copy span
	// attributes into resource attributes; see spanmappers.go.
	mountSpanMappers(app)
	// And the eleven that cannot be typed, each named and justified.
	mountHatches(app)
	return nil
}

// mountHatches registers the ELEVEN routes that cannot be typed ops, one route
// literal each, with the reason next to it. This list is meant to shrink and is
// meant to be hard to grow: adding to it costs a justification in review, where
// a catch-all cost nothing.
//
// A typed op declares an input type, an output type and a success status, and
// zip projects that declaration into the OpenAPI document, the MCP tool, the CLI
// command and the by-name call plane. Every route here breaks that declaration
// in a way that would make the document LIE, and lying in the document is worse
// than being absent from it — a generated client that trusts a false contract
// fails at the customer, not at review.
//
// Two of these eleven were not reachable AT ALL from the composed binary before
// this pass: /ws/query_progress and the two /v1/sentry ingest routes sit outside
// /v1/o11y, so the old catch-all never saw them. That is the second thing a
// wildcard hides — not just which routes are un-typed, but which are missing.
func mountHatches(app *zip.App) {
	h := zip.AdaptNetHTTP(handlerAdapter{})

	// ── 1. STREAMS: there is no one answer to name ───────────────────────────
	// These never produce a single complete JSON value. relay buffers a whole
	// answer through an httptest recorder before decoding it, so a typed
	// livetail would hang on the first tail, and a typed progress poll would
	// return only after the query it reports on had already finished — which is
	// the one thing a progress endpoint must not do.
	app.Get(o11yRoot+"/logs/livetail", h)    // unbounded stream of log records
	app.Get(o11yRoot+"/query_progress", h)   // long-poll: holds the connection until the next tick
	app.Get("/ws/query_progress", h)         // the same read over a websocket; the Upgrade IS the contract
	app.Post(o11yRoot+"/export_raw_data", h) // chunked CSV/JSONL attachment, X-Response-Complete trailer

	// ── 2. REDIRECTS: the answer is a Location, not a body ───────────────────
	// The three sign-in callbacks answer 303 with a Location header and no
	// payload. A typed op declares a 2xx JSON contract, so typing these would
	// publish a response schema for a response that does not exist, and hide the
	// header that is the entire point of the call.
	app.Get(o11yRoot+"/complete/google", h) // Google OIDC callback → 303 to the console
	app.Get(o11yRoot+"/complete/oidc", h)   // generic OIDC callback → 303
	app.Post(o11yRoot+"/complete/saml", h)  // SAML assertion consumer → 303

	// ── 3. A FOREIGN PROTOCOL WE RECEIVE ─────────────────────────────────────
	// Sentry-compatible ingest. The body is an application/x-sentry-envelope
	// frame, not JSON, and the caller is a Sentry SDK authenticating with a DSN
	// public key rather than a Hanzo session. The /api/ segment is NOT ours to
	// name: an SDK appends its own fixed /api/<project>/envelope/ suffix to
	// whatever DSN path it is given, so renaming it would break every SDK in the
	// field. We RECEIVE this shape; we do not publish it.
	app.Post(o11yRoot+"/api/:project_id/envelope/", h) // Sentry envelope ingest
	app.Post(o11yRoot+"/api/:project_id/store/", h)    // legacy single-event ingest
	app.Post(sentryRoot+"/:project/envelope/", h)      // the same wire on the clean /v1/sentry root
	app.Post(sentryRoot+"/:project/store/", h)         // same
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
