package app

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"slices"

	published "github.com/hanzoai/o11y"
	"github.com/hanzoai/o11y/pkg/errors"
	"github.com/hanzoai/o11y/pkg/queryparser"

	"github.com/soheilhy/cmux"
	"github.com/zap-proto/fiber/v3"
	"github.com/zap-proto/fiber/v3/middleware/adaptor"
	"github.com/zap-proto/fiber/v3/middleware/compress"
	"github.com/zap-proto/fiber/v3/middleware/cors"
	"github.com/zap-proto/zip"

	"github.com/hanzoai/o11y/pkg/http/handler"
	"github.com/hanzoai/o11y/pkg/http/middleware"
	"github.com/hanzoai/o11y/pkg/http/routing"
	"github.com/hanzoai/o11y/pkg/licensing/nooplicensing"
	"github.com/hanzoai/o11y/pkg/o11y"
	"github.com/hanzoai/o11y/pkg/query-service/agentConf"
	"github.com/hanzoai/o11y/pkg/query-service/app/datastorereader"
	"github.com/hanzoai/o11y/pkg/query-service/app/integrations"
	"github.com/hanzoai/o11y/pkg/query-service/app/logparsingpipeline"
	"github.com/hanzoai/o11y/pkg/query-service/app/opamp"
	opAmpModel "github.com/hanzoai/o11y/pkg/query-service/app/opamp/model"
	"github.com/hanzoai/o11y/pkg/types/coretypes"
	"github.com/hanzoai/o11y/pkg/web"

	"log/slog"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/propagation"

	"github.com/hanzoai/o11y/pkg/query-service/constants"
	"github.com/hanzoai/o11y/pkg/query-service/healthcheck"
	"github.com/hanzoai/o11y/pkg/query-service/utils"
)

// Server runs HTTP, Mux and a grpc server
type Server struct {
	config o11y.Config
	o11y   *o11y.O11y

	// public http router
	httpConn     net.Listener
	app          *zip.App
	routes       *routing.Table
	httpHostPort string

	opampServer *opamp.Server

	unavailableChannel chan healthcheck.Status
}

// NewServer creates and initializes Server
func NewServer(config o11y.Config, o11y *o11y.O11y) (*Server, error) {
	integrationsController, err := integrations.NewController(o11y.SQLStore, o11y.Modules.Dashboard)
	if err != nil {
		return nil, err
	}

	reader := datastorereader.NewReader(
		o11y.Instrumentation.Logger(),
		o11y.SQLStore,
		o11y.TelemetryStore,
		o11y.Prometheus,
		o11y.TelemetryStore.Cluster(),
		o11y.Cache,
		o11y.Flagger,
		nil,
	)

	logParsingPipelineController, err := logparsingpipeline.NewLogParsingPipelinesController(
		o11y.SQLStore,
		integrationsController.GetPipelinesForInstalledIntegrations,
		reader,
		o11y.Flagger,
	)
	if err != nil {
		return nil, err
	}

	apiHandler, err := NewAPIHandler(APIHandlerOpts{
		Reader:                        reader,
		IntegrationsController:        integrationsController,
		LogsParsingPipelineController: logParsingPipelineController,
		FluxInterval:                  config.Querier.FluxInterval,
		LicensingAPI:                  nooplicensing.NewLicenseAPI(),
		O11y:                          o11y,
		QueryParserAPI:                queryparser.NewAPI(o11y.Instrumentation.ToProviderSettings(), o11y.QueryParser),
	}, config)
	if err != nil {
		return nil, err
	}

	s := &Server{
		config:             config,
		o11y:               o11y,
		httpHostPort:       constants.HTTPHostPort,
		unavailableChannel: make(chan healthcheck.Status),
	}

	app, err := s.createPublicServer(apiHandler, o11y.Web)

	if err != nil {
		return nil, err
	}

	s.app = app

	opAmpModel.Init(o11y.SQLStore, o11y.Instrumentation.Logger(), o11y.Modules.OrgGetter)

	agentConfMgr, err := agentConf.Initiate(
		&agentConf.ManagerOptions{
			Store: o11y.SQLStore,
			AgentFeatures: []agentConf.AgentFeature{
				logParsingPipelineController,
				o11y.Modules.SpanMapper,
				o11y.Modules.LLMPricingRule,
			},
		},
	)
	if err != nil {
		return nil, err
	}

	s.opampServer = opamp.InitializeServer(
		&opAmpModel.AllAgents,
		agentConfMgr,
		o11y.Instrumentation,
	)

	return s, nil
}

// HealthCheckStatus returns health check status channel a client can subscribe to
func (s Server) HealthCheckStatus() chan healthcheck.Status {
	return s.unavailableChannel
}

// PublicHandler returns the fully-wired public HTTP surface as a net/http
// handler — every middleware (IdentN identity resolution over the
// gateway-injected Hanzo IAM session headers X-Org-Id/X-User-Id/X-User-Email,
// AuthZ, audit, timeout, recovery), serving the public paths verbatim — WITHOUT
// binding a listener. It lets an embedding host (the unified cloud binary) serve
// /v1/o11y/* on its own HTTP stack instead of running a second Deployment;
// Start/initListeners stay the standalone entrypoints.
//
// STREAMS SURVIVE THIS BRIDGE, ONE UPGRADE DOES NOT. The bridge pumps a
// streamed answer straight through when the host's writer is an http.Flusher —
// which it is, for every host that reaches this over zip — so livetail, the
// long-poll and the chunked export all stream end to end. A CONNECTION HIJACK
// does not survive it: the websocket at /ws/query_progress hijacks the
// connection, and a hijack handed to a request context that no server is
// driving is dropped. A host that needs that route serves this Server's own
// listener (Start) rather than embedding it, which is what the standalone
// deployment does.
func (s *Server) PublicHandler() http.Handler {
	return adaptor.FiberApp(s.app.Fiber())
}

// createPublicServer builds the ONE router this service serves.
//
// Every route on it is registered through routing.Router — the API tree of
// pkg/apiserver/o11yapiserver and the query-service's own — so there is one
// router, one param model and one middleware chain for all 367 of them.
//
// THE CHAIN IS COMPOSED AT THE LEAF, not registered as ambient middleware, and
// that is deliberate. The middleware here is net/http middleware, and two
// members of it need the ROUTE: Audit keys its record on the path template, and
// Resource resolves the route's declared resources. Ambient middleware runs
// BEFORE the router has matched anything, so those two would read an empty
// route. Composing the chain around each leaf reproduces exactly the order the
// tree this replaces ran in — match first, then middleware, then handler — and
// hands Resource its defs from the registration that declared them instead of
// making it ask the router what it just matched.
func (s *Server) createPublicServer(api *APIHandler, web web.Web) (*zip.App, error) {
	app := zip.New(zip.Config{
		AppName:               "o11y",
		DisableStartupMessage: true,
	})

	// Compression and CORS are ambient — they apply to every answer including
	// the console's, and CORS must answer a preflight for a method no route
	// takes. They run outside the router, in the order they always have:
	// compress outermost, then CORS, then the match.
	app.Fiber().Use(compress.New())
	app.Fiber().Use(cors.New(cors.Config{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{fiber.MethodGet, fiber.MethodDelete, fiber.MethodPost, fiber.MethodPut, fiber.MethodPatch, fiber.MethodOptions},
		AllowHeaders: []string{"Accept", "Authorization", "Content-Type", "cache-control", "X-O11Y-QUERY-ID", "Sec-WebSocket-Protocol"},
	}))

	recovery := middleware.NewRecovery(s.o11y.Instrumentation.Logger())
	identn := middleware.NewIdentN(s.o11y.IdentNResolver, s.o11y.Sharder, s.o11y.Instrumentation.Logger())
	timeout := middleware.NewTimeout(s.o11y.Instrumentation.Logger(),
		s.config.APIServer.Timeout.ExcludedRoutes,
		s.config.APIServer.Timeout.Default,
		s.config.APIServer.Timeout.Max,
	)
	resource := middleware.NewResource(s.o11y.Instrumentation.Logger())
	audit := middleware.NewAudit(s.o11y.Instrumentation.Logger(), s.config.APIServer.Logging.ExcludedRoutes, s.o11y.Auditor)
	comment := middleware.NewComment()
	// Server tracing reads the route TEMPLATE off the request the router bound
	// it to, so a span is one operation and not one per id. The filter is the
	// health probe, as before: a liveness check every second is not a trace.
	tracing := func(next http.Handler) http.Handler {
		return otelhttp.NewHandler(next, "apiserver",
			otelhttp.WithMeterProvider(s.o11y.Instrumentation.MeterProvider()),
			otelhttp.WithTracerProvider(s.o11y.Instrumentation.TracerProvider()),
			otelhttp.WithPropagators(propagation.NewCompositeTextMapPropagator(propagation.Baggage{}, propagation.TraceContext{})),
			otelhttp.WithFilter(func(r *http.Request) bool {
				return !slices.Contains([]string{"/v1/o11y/health"}, r.URL.Path)
			}),
			otelhttp.WithSpanNameFormatter(func(operation string, r *http.Request) string {
				if path := coretypes.RoutePath(r); path != "" {
					return r.Method + " " + path
				}
				return operation
			}),
		)
	}

	chain := func(defs []handler.ResourceDef) func(http.Handler) http.Handler {
		wrapResource := resource.For(defs)
		return func(next http.Handler) http.Handler {
			return recovery.Wrap(tracing(identn.Wrap(timeout.Wrap(wrapResource(audit.Wrap(comment.Wrap(next)))))))
		}
	}

	// Group("") is the App as a zip.Router with no prefix — the root every route
	// on this service hangs off, and the only place a prefix is not stated.
	r := routing.New(app.Group(""), chain)
	am := middleware.NewAuthZ(s.o11y.Instrumentation.Logger(), s.o11y.Modules.OrgGetter, s.o11y.Authz)

	api.RegisterRoutes(r, am)
	api.RegisterLogsRoutes(r, am)
	api.RegisterIntegrationRoutes(r, am)
	api.RegisterQueryRangeV3Routes(r, am)
	api.RegisterInfraMetricsRoutes(r, am)
	api.RegisterWebSocketPaths(r, am)
	api.RegisterQueryRangeV4Routes(r, am)
	api.RegisterMessagingQueuesRoutes(r, am)
	api.RegisterThirdPartyApiRoutes(r, am)
	api.RegisterTraceFunnelsRoutes(r, am)

	s.o11y.APIServer.AddToRouter(r)
	s.routes = r.Table()

	// The module's own declaration of this surface, and its projections. After the
	// service's own routes, before the console catch-all — see publish.
	if err := publish(app); err != nil {
		return nil, err
	}

	// The console is the TERMINAL route, registered after every API route so the
	// router's in-order match gives the API precedence and only paths that match
	// nothing else reach the SPA shell. web is a plain http.Handler — pkg/web is
	// router-agnostic — and it is served through the same chain every route is,
	// so a panic in it is still recovered and a request for it is still audited,
	// exactly as when it was the last route on the tree this replaces. It is
	// unconditional: the null provider answers 404, so a headless deployment
	// (web.enabled=false) serves exactly what an unregistered route served.
	app.All("/*", zip.AdaptNetHTTP(chain(nil)(web)))

	// No prefix stripping. Every route is registered at its full public path
	// (/v1/o11y/…, /v1/sentry/…), so the request path that arrives is the path that
	// matches — standalone and embedded in the cloud binary alike. The former
	// StripPrefix(Global.ExternalPath()) wrapper predates that: it existed to graft a
	// mount prefix onto routes registered at bare names, and once the names carried
	// the prefix it could only subtract it back off (/v1/o11y/services → /services),
	// which the web catch-all then answered with index.html — a blank console, not
	// even a 404. ExternalPath survives where it belongs: building absolute URLs
	// (cookie Path, OAuth redirect, SPA base href), never routing.
	return app, nil
}

// publish installs THE PUBLISHED TABLE — the module's own declaration of this
// service's surface, github.com/hanzoai/o11y — onto the router this binary
// serves, then installs zip's projections of it.
//
// The registrations above publish's call site are the service's IMPLEMENTATION:
// real handlers, one per route. The table is the DECLARATION of the same 367:
// 353 typed ops that each carry a named input, a named output and their prose,
// 11 named escape hatches with the reason each cannot be typed, and the 3
// probes. It was written, tested and shipped — and no binary imported it. The
// standalone image builds ./cmd/community, and the table lived in a package
// braided to the embedding host's dependency struct (hanzoai/cloud's Deps, for a
// logger), so the community graph could not reach it without dragging the host
// in. It did not, so 353 published operations and 353 MCP tools were true of the
// source and absent from the process. Un-braiding Mount is what let this line
// exist; this line is what puts the conversion in the binary.
//
// ORDER IS THE CONTRACT, twice over:
//
//   - AFTER the service's own routes. Fiber matches in registration order, so
//     every one of these paths is still answered by the handler that has always
//     answered it — no relay hop, no buffered round-trip, and therefore livetail,
//     the long-poll and the chunked export still stream on this listener. The
//     table names the surface here; it does not stand in front of it.
//   - BEFORE the console's terminal catch-all. Build's routes are ordinary
//     routes; registered after /* the SPA would answer for them.
//
// Build is what turns the in-memory registry into doors: the OpenAPI document
// at /.well-known/openapi.json, /docs, the MCP tool surface at /mcp and the
// by-name call plane. zip defers it so a host can finish mounting first, and
// Listen calls it — but this server serves through Fiber().Listener rather than
// zip's own Listen, so without this call the document would exist in the process
// and answer on no port. It is guarded to run once however it is reached.
//
// It RETURNS the verdict, and this returns it onward: a composition that does not
// compose — two definitions claiming one address, a cycle — fails the boot that
// mounted it rather than serving a router silently missing the doors.
func publish(app *zip.App) error {
	if err := published.Mount(app); err != nil {
		return err
	}
	return app.Build()
}

// initListeners initialises listeners of the server
func (s *Server) initListeners() error {
	// listen on public port
	var err error
	publicHostPort := s.httpHostPort
	if publicHostPort == "" {
		return fmt.Errorf("constants.HTTPHostPort is required")
	}

	s.httpConn, err = net.Listen("tcp", publicHostPort)
	if err != nil {
		return err
	}

	slog.Info(fmt.Sprintf("Query server started listening on %s...", s.httpHostPort))

	return nil
}

// Start listening on http and private http port concurrently
func (s *Server) Start(ctx context.Context) error {
	err := s.initListeners()
	if err != nil {
		return err
	}

	var httpPort int
	if port, err := utils.GetPort(s.httpConn.Addr()); err == nil {
		httpPort = port
	}

	go func() {
		slog.Info("Starting HTTP server", "port", httpPort, "addr", s.httpHostPort)

		switch err := s.app.Fiber().Listener(s.httpConn, fiber.ListenConfig{DisableStartupMessage: true}); err {
		case nil, http.ErrServerClosed, cmux.ErrListenerClosed:
			// normal exit, nothing to do
		default:
			slog.Error("Could not start HTTP server", errors.Attr(err))
		}
		s.unavailableChannel <- healthcheck.Unavailable
	}()

	go func() {
		slog.Info("Starting OpAmp Websocket server", "addr", constants.OpAmpWsEndpoint)
		err := s.opampServer.Start(constants.OpAmpWsEndpoint)
		if err != nil {
			slog.Error("opamp ws server failed to start", errors.Attr(err))
			s.unavailableChannel <- healthcheck.Unavailable
		}
	}()

	return nil
}

func (s *Server) Stop(ctx context.Context) error {
	if s.app != nil {
		if err := s.app.ShutdownWithContext(ctx); err != nil {
			return err
		}
	}

	s.opampServer.Stop()

	return nil
}
