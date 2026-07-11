package app

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"slices"
	"strings"

	"github.com/hanzoai/o11y/pkg/errors"
	"github.com/hanzoai/o11y/pkg/queryparser"

	"github.com/gorilla/handlers"

	"github.com/rs/cors"
	"github.com/soheilhy/cmux"

	"github.com/hanzoai/o11y/pkg/apiserver/o11yapiserver"
	"github.com/hanzoai/o11y/pkg/http/middleware"
	"github.com/hanzoai/o11y/pkg/licensing/nooplicensing"
	"github.com/hanzoai/o11y/pkg/o11y"
	"github.com/hanzoai/o11y/pkg/query-service/agentConf"
	"github.com/hanzoai/o11y/pkg/query-service/app/clickhouseReader"
	"github.com/hanzoai/o11y/pkg/query-service/app/integrations"
	"github.com/hanzoai/o11y/pkg/query-service/app/logparsingpipeline"
	"github.com/hanzoai/o11y/pkg/query-service/app/opamp"
	opAmpModel "github.com/hanzoai/o11y/pkg/query-service/app/opamp/model"
	"github.com/hanzoai/o11y/pkg/web"

	"log/slog"

	"go.opentelemetry.io/contrib/instrumentation/github.com/gorilla/mux/otelmux"
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
	httpServer   *http.Server
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

	reader := clickhouseReader.NewReader(
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

	httpServer, err := s.createPublicServer(apiHandler, o11y.Web)

	if err != nil {
		return nil, err
	}

	s.httpServer = httpServer

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

// PublicHandler returns the fully-wired public HTTP handler — every middleware
// (IdentN identity resolution over the gateway-injected Hanzo IAM session
// headers X-Org-Id/X-User-Id/X-User-Email, AuthZ, audit, timeout, recovery) and
// the ExternalPath strip — WITHOUT binding a listener. It lets an embedding host
// (the unified cloud binary) serve /v1/o11y/* on its own HTTP stack instead of
// running a second Deployment; Start/initListeners stay the standalone entrypoints.
func (s *Server) PublicHandler() http.Handler {
	return s.httpServer.Handler
}

func (s *Server) createPublicServer(api *APIHandler, web web.Web) (*http.Server, error) {
	r := NewRouter()

	r.Use(middleware.NewRecovery(s.o11y.Instrumentation.Logger()).Wrap)
	r.Use(otelmux.Middleware(
		"apiserver",
		otelmux.WithMeterProvider(s.o11y.Instrumentation.MeterProvider()),
		otelmux.WithTracerProvider(s.o11y.Instrumentation.TracerProvider()),
		otelmux.WithPropagators(propagation.NewCompositeTextMapPropagator(propagation.Baggage{}, propagation.TraceContext{})),
		otelmux.WithFilter(func(r *http.Request) bool {
			return !slices.Contains([]string{"/api/v1/health"}, r.URL.Path)
		}),
	))
	r.Use(middleware.NewIdentN(s.o11y.IdentNResolver, s.o11y.Sharder, s.o11y.Instrumentation.Logger()).Wrap)
	r.Use(middleware.NewTimeout(s.o11y.Instrumentation.Logger(),
		s.config.APIServer.Timeout.ExcludedRoutes,
		s.config.APIServer.Timeout.Default,
		s.config.APIServer.Timeout.Max,
	).Wrap)
	r.Use(middleware.NewResource(s.o11y.Instrumentation.Logger()).Wrap)
	r.Use(middleware.NewAudit(s.o11y.Instrumentation.Logger(), s.config.APIServer.Logging.ExcludedRoutes, s.o11y.Auditor).Wrap)
	r.Use(middleware.NewComment().Wrap)

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

	err := s.o11y.APIServer.AddToRouter(r)
	if err != nil {
		return nil, err
	}

	c := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "DELETE", "POST", "PUT", "PATCH", "OPTIONS"},
		AllowedHeaders: []string{"Accept", "Authorization", "Content-Type", "cache-control", "X-O11Y-QUERY-ID", "Sec-WebSocket-Protocol"},
	})

	handler := c.Handler(r)

	handler = handlers.CompressHandler(handler)

	err = web.AddToRouter(r)
	if err != nil {
		return nil, err
	}

	// Register the version-less /api/<resource> aliases for every /api/vN route on the
	// assembled router, so the ONE public Hanzo contract — /v1/o11y/<resource> (rewritten
	// to /api/<resource> by mount.go rewriteExternalPath) — resolves. Done HERE, after ALL
	// routes (api + web) are registered on r, matching what the o11yapiserver provider does.
	// Without it the embedded runtime (cloud one-binary via community.NewServer) 404s every
	// version-less call — the console's canonical o11y contract. Additive: adds aliases only.
	if err = o11yapiserver.AddVersionlessAliases(r); err != nil {
		return nil, err
	}

	routePrefix := s.config.Global.ExternalPath()
	if routePrefix != "" {
		prefixed := http.StripPrefix(routePrefix, handler)
		handler = http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			// A path already under /api/ reaches the router directly. In the embedded
			// one-binary runtime the host (cloud mount.go rewriteExternalPath) rewrote
			// /v1/o11y/<res> → /api/<res> BEFORE this handler, so StripPrefix(routePrefix=
			// /v1/o11y) would 404 it — the /v1/o11y prefix is already gone (double-strip),
			// which broke EVERY embedded o11y data call. Standalone requests still arrive
			// /v1/o11y-prefixed and fall through to StripPrefix. Supersedes the prior
			// health-only special-case (health is under /api/ too).
			if strings.HasPrefix(req.URL.Path, "/api/") {
				r.ServeHTTP(w, req)
				return
			}

			// Hanzo Sentry is served under the CLEAN /v1/sentry contract (no /api/,
			// no /v1/o11y rewrite): its routes are registered on r at their literal
			// /v1/sentry/… path. StripPrefix(routePrefix=/v1/o11y) would 404 them, so
			// pass them straight to the router — the same escape hatch the /api/ paths
			// use, for the same reason.
			if strings.HasPrefix(req.URL.Path, "/v1/sentry/") {
				r.ServeHTTP(w, req)
				return
			}

			prefixed.ServeHTTP(w, req)
		})
	}

	return &http.Server{
		Handler: handler,
	}, nil
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

		switch err := s.httpServer.Serve(s.httpConn); err {
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
	if s.httpServer != nil {
		if err := s.httpServer.Shutdown(context.Background()); err != nil {
			return err
		}
	}

	s.opampServer.Stop()

	return nil
}
