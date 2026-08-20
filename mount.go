package o11y

import (
	"net/http"

	"github.com/zap-proto/zip"
)

// Mount registers Hanzo o11y's HTTP surface under /v1/o11y per HIP-0106.
//
// A ROUTE TABLE IS A VALUE, AND IT TAKES THE ROUTER AND NOTHING ELSE. Mount used
// to take a second argument — the embedding host's dependency struct,
// hanzoai/cloud's Deps — and it read exactly ONE field of it, Logger, on exactly
// one line, to announce itself. That one line braided WHAT the routes are into
// WHICH host serves them, and the price was paid three levels up:
//
//   - github.com/hanzoai/o11y required github.com/hanzoai/cloud, which requires
//     github.com/hanzoai/o11y. A module CYCLE, for a log line.
//   - Because the cycle made `go mod download` unresolvable in an image that has
//     no cloud checkout, the community Dockerfile had to build one package
//     instead of the module, and that package — cmd/community — could not import
//     these declarations without dragging the host in. So it did not import them
//     at all, and the whole conversion (353 typed ops, 353 published operations,
//     353 MCP tools) shipped in a package the running process never linked.
//
// The router already carries a logger: the host configured it with its own when
// it built the app, so app.Logger() is the SAME value Deps.Logger was, reached
// through the argument that was already here. Nothing was carried by the second
// argument that the first did not already have.
//
// With it gone the declarations stand alone, and BOTH hosts mount this ONE
// table: the community server's own public router (pkg/query-service/app, which
// is also the handler the unified binary embeds) and hanzoai/cloud's outer
// router. One declaration, two hosts, nothing to drift.
//
// The o11y runtime (metrics, traces, logs, dashboards, alerts) is heavy:
// telemetry stores, rule manager, websocket attachments, opamp server. These
// routes do not re-implement it — each hands its call to the runtime's handler
// FOR ITS OWN ADDRESS (see [SetRuntime] and relay), and until a runtime is
// installed they 503 with a clear error rather than pretending.
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
// A HOST MAY TAKE AN ADDRESS. Since every route is named there is no wildcard
// left for a host's own route to shadow, so a host that serves one of these
// addresses itself has to say so with [Claimed] — see claim.go for why that is a
// statement of ownership rather than a filter. Without one, nothing changes and
// the whole table is declared.
func Mount(app *zip.App, opts ...Option) error {
	c := new(conf)
	for _, o := range opts {
		o(c)
	}
	// Read by the declaration verbs below, for this call only: Mount is the
	// composition root, so the set is written once here and every declaration it
	// makes happens before it returns.
	mounting = c
	defer func() { mounting = nil }()

	app.Logger().Info("o11y: mounting routes", "prefix", o11yRoot, "claimed", len(c.claimed))

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
// this pass: /ws/query_progress and the two /v1/sentinel ingest routes sit outside
// /v1/o11y, so the old catch-all never saw them. That is the second thing a
// wildcard hides — not just which routes are un-typed, but which are missing.
func mountHatches(app *zip.App) {
	// ── 1. STREAMS: there is no one answer to name ───────────────────────────
	// These never produce a single complete JSON value. relay buffers a whole
	// answer through an httptest recorder before decoding it, so a typed
	// livetail would hang on the first tail, and a typed progress poll would
	// return only after the query it reports on had already finished — which is
	// the one thing a progress endpoint must not do.
	route(app, http.MethodGet, o11yRoot+"/logs/livetail")    // unbounded stream of log records
	route(app, http.MethodGet, o11yRoot+"/query_progress")   // long-poll: holds the connection until the next tick
	route(app, http.MethodGet, "/ws/query_progress")         // the same read over a websocket; the Upgrade IS the contract
	route(app, http.MethodPost, o11yRoot+"/export_raw_data") // chunked CSV/JSONL attachment, X-Response-Complete trailer

	// ── 2. REDIRECTS: the answer is a Location, not a body ───────────────────
	// The three sign-in callbacks answer 303 with a Location header and no
	// payload. A typed op declares a 2xx JSON contract, so typing these would
	// publish a response schema for a response that does not exist, and hide the
	// header that is the entire point of the call.
	route(app, http.MethodGet, o11yRoot+"/complete/google") // Google OIDC callback → 303 to the console
	route(app, http.MethodGet, o11yRoot+"/complete/oidc")   // generic OIDC callback → 303
	route(app, http.MethodPost, o11yRoot+"/complete/saml")  // SAML assertion consumer → 303

	// ── 3. A FOREIGN PROTOCOL WE RECEIVE ─────────────────────────────────────
	// Sentry-compatible ingest. The body is an application/x-sentry-envelope
	// frame, not JSON, and the caller is a Sentry SDK authenticating with a DSN
	// public key rather than a Hanzo session. The /api/ segment is NOT ours to
	// name: an SDK appends its own fixed /api/<project>/envelope/ suffix to
	// whatever DSN path it is given, so renaming it would break every SDK in the
	// field. We RECEIVE this shape; we do not publish it.
	route(app, http.MethodPost, o11yRoot+"/api/:project_id/envelope/") // Sentry envelope ingest
	route(app, http.MethodPost, o11yRoot+"/api/:project_id/store/")    // legacy single-event ingest
	route(app, http.MethodPost, sentinelRoot+"/:project/envelope/")      // the same wire on the clean /v1/sentinel root
	route(app, http.MethodPost, sentinelRoot+"/:project/store/")         // same
}
