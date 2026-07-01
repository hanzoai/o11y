package app

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"slices"

	"github.com/hanzoai/o11y/pkg/cache/memorycache"
	"github.com/hanzoai/o11y/pkg/factory"
	"github.com/hanzoai/o11y/pkg/queryparser"
	"github.com/hanzoai/o11y/pkg/ruler/rulestore/sqlrulestore"
	"github.com/hanzoai/o11y/pkg/types/telemetrytypes"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gorilla/mux/otelmux"
	"go.opentelemetry.io/otel/propagation"

	"github.com/hanzoai/o11y/pkg/cache/memorycache"
	"github.com/hanzoai/o11y/pkg/errors"

	"github.com/gorilla/handlers"

	"github.com/hanzoai/o11y/ee/query-service/app/api"
	"github.com/hanzoai/o11y/ee/query-service/rules"
	"github.com/hanzoai/o11y/ee/query-service/usage"
	"github.com/hanzoai/o11y/pkg/alertmanager"
	"github.com/hanzoai/o11y/pkg/cache"
	"github.com/hanzoai/o11y/pkg/http/middleware"
	"github.com/hanzoai/o11y/pkg/modules/organization"
	"github.com/hanzoai/o11y/pkg/prometheus"
	"github.com/hanzoai/o11y/pkg/querier"
	"github.com/hanzoai/o11y"
	"github.com/hanzoai/o11y/pkg/sqlstore"
	"github.com/hanzoai/o11y/pkg/telemetrystore"
	"github.com/hanzoai/o11y/pkg/web"
	"github.com/rs/cors"
	"github.com/soheilhy/cmux"

	"github.com/hanzoai/o11y/pkg/query-service/agentConf"
	baseapp "github.com/hanzoai/o11y/pkg/query-service/app"
	"github.com/hanzoai/o11y/pkg/query-service/app/clickhouseReader"
	"github.com/hanzoai/o11y/pkg/query-service/app/cloudintegrations"
	"github.com/hanzoai/o11y/pkg/query-service/app/integrations"
	"github.com/hanzoai/o11y/pkg/query-service/app/logparsingpipeline"
	"github.com/hanzoai/o11y/pkg/query-service/app/opamp"
	opAmpModel "github.com/hanzoai/o11y/pkg/query-service/app/opamp/model"
	baseconst "github.com/hanzoai/o11y/pkg/query-service/constants"
	"github.com/hanzoai/o11y/pkg/query-service/healthcheck"
	baseint "github.com/hanzoai/o11y/pkg/query-service/interfaces"
	baserules "github.com/hanzoai/o11y/pkg/query-service/rules"
	"github.com/hanzoai/o11y/pkg/query-service/utils"
	"go.uber.org/zap"
)

// Server runs HTTP, Mux and a grpc server
type Server struct {
	config      o11y.Config
	o11y      *o11y.HanzoO11y
	ruleManager *baserules.Manager

	// public http router
	httpConn     net.Listener
	httpServer   *http.Server
	httpHostPort string

	opampServer *opamp.Server

	// Usage manager
	usageManager *usage.Manager

	unavailableChannel chan healthcheck.Status
}

// NewServer creates and initializes Server
func NewServer(config o11y.Config, o11y *o11y.HanzoO11y) (*Server, error) {
	cacheForTraceDetail, err := memorycache.New(context.TODO(), o11y.Instrumentation.ToProviderSettings(), cache.Config{
		Provider: "memory",
		Memory: cache.Memory{
			NumCounters: 10 * 10000,
			MaxCost:     1 << 27, // 128 MB
		},
	})
	if err != nil {
		return nil, err
	}

	reader := clickhouseReader.NewReader(
		o11y.SQLStore,
		o11y.TelemetryStore,
		o11y.Prometheus,
		o11y.TelemetryStore.Cluster(),
		config.Querier.FluxInterval,
		cacheForTraceDetail,
		o11y.Cache,
		nil,
	)

	rm, err := makeRulesManager(
		reader,
		o11y.Cache,
		o11y.Alertmanager,
		o11y.SQLStore,
		o11y.TelemetryStore,
		o11y.TelemetryMetadataStore,
		o11y.Prometheus,
		o11y.Modules.OrgGetter,
		o11y.Querier,
		o11y.Instrumentation.ToProviderSettings(),
		o11y.QueryParser,
	)

	if err != nil {
		return nil, err
	}

	// initiate opamp
	opAmpModel.Init(o11y.SQLStore, o11y.Instrumentation.Logger(), o11y.Modules.OrgGetter)

	integrationsController, err := integrations.NewController(o11y.SQLStore)
	if err != nil {
		return nil, fmt.Errorf(
			"couldn't create integrations controller: %w", err,
		)
	}

	cloudIntegrationsController, err := cloudintegrations.NewController(o11y.SQLStore)
	if err != nil {
		return nil, fmt.Errorf(
			"couldn't create cloud provider integrations controller: %w", err,
		)
	}

	// ingestion pipelines manager
	logParsingPipelineController, err := logparsingpipeline.NewLogParsingPipelinesController(
		o11y.SQLStore,
		integrationsController.GetPipelinesForInstalledIntegrations,
		reader,
		signoz.Flagger,
	)
	if err != nil {
		return nil, err
	}

	// initiate agent config handler
	agentConfMgr, err := agentConf.Initiate(&agentConf.ManagerOptions{
		Store:         o11y.SQLStore,
		AgentFeatures: []agentConf.AgentFeature{logParsingPipelineController},
	})
	if err != nil {
		return nil, err
	}

	// start the usagemanager
	usageManager, err := usage.New(o11y.Licensing, o11y.TelemetryStore.ClickhouseDB(), o11y.Zeus, o11y.Modules.OrgGetter)
	if err != nil {
		return nil, err
	}
	err = usageManager.Start(context.Background())
	if err != nil {
		return nil, err
	}

	apiOpts := api.APIHandlerOptions{
		DataConnector:                 reader,
		UsageManager:                  usageManager,
		IntegrationsController:        integrationsController,
		LogsParsingPipelineController: logParsingPipelineController,
		FluxInterval:                  config.Querier.FluxInterval,
		GatewayUrl:                    config.Gateway.URL.String(),
		GlobalConfig:                  config.Global,
	}

	apiHandler, err := api.NewAPIHandler(apiOpts, o11y, config)
	if err != nil {
		return nil, err
	}

	s := &Server{
		config:             config,
		o11y:             o11y,
		ruleManager:        rm,
		httpHostPort:       baseconst.HTTPHostPort,
		unavailableChannel: make(chan healthcheck.Status),
		usageManager:       usageManager,
	}

	httpServer, err := s.createPublicServer(apiHandler, o11y.Web)

	if err != nil {
		return nil, err
	}

	s.httpServer = httpServer

	s.opampServer = opamp.InitializeServer(
		&opAmpModel.AllAgents, agentConfMgr, o11y.Instrumentation,
	)

	return s, nil
}

// HealthCheckStatus returns health check status channel a client can subscribe to
func (s Server) HealthCheckStatus() chan healthcheck.Status {
	return s.unavailableChannel
}

func (s *Server) createPublicServer(apiHandler *api.APIHandler, web web.Web) (*http.Server, error) {
	r := baseapp.NewRouter()
	am := middleware.NewAuthZ(s.o11y.Instrumentation.Logger(), s.o11y.Modules.OrgGetter, s.o11y.Authz)

	r.Use(middleware.NewRecovery(s.signoz.Instrumentation.Logger()).Wrap)
	r.Use(otelmux.Middleware(
		"apiserver",
		otelmux.WithMeterProvider(s.o11y.Instrumentation.MeterProvider()),
		otelmux.WithTracerProvider(s.o11y.Instrumentation.TracerProvider()),
		otelmux.WithPropagators(propagation.NewCompositeTextMapPropagator(propagation.Baggage{}, propagation.TraceContext{})),
		otelmux.WithFilter(func(r *http.Request) bool {
			return !slices.Contains([]string{"/v1/o11y/v1/health"}, r.URL.Path)
		}),
	))
	r.Use(middleware.NewAuthN([]string{"Authorization", "Sec-WebSocket-Protocol"}, s.o11y.Sharder, s.o11y.Tokenizer, s.o11y.Instrumentation.Logger()).Wrap)
	r.Use(middleware.NewAPIKey(s.o11y.SQLStore, []string{"HANZO-API-KEY"}, s.o11y.Instrumentation.Logger(), s.o11y.Sharder).Wrap)
	r.Use(middleware.NewTimeout(s.o11y.Instrumentation.Logger(),
		s.config.APIServer.Timeout.ExcludedRoutes,
		s.config.APIServer.Timeout.Default,
		s.config.APIServer.Timeout.Max,
	).Wrap)
	r.Use(middleware.NewLogging(s.o11y.Instrumentation.Logger(), s.config.APIServer.Logging.ExcludedRoutes).Wrap)
	r.Use(middleware.NewComment().Wrap)

	apiHandler.RegisterRoutes(r, am)
	apiHandler.RegisterLogsRoutes(r, am)
	apiHandler.RegisterIntegrationRoutes(r, am)
	apiHandler.RegisterQueryRangeV3Routes(r, am)
	apiHandler.RegisterInfraMetricsRoutes(r, am)
	apiHandler.RegisterQueryRangeV4Routes(r, am)
	apiHandler.RegisterWebSocketPaths(r, am)
	apiHandler.RegisterMessagingQueuesRoutes(r, am)
	apiHandler.RegisterThirdPartyApiRoutes(r, am)
	apiHandler.RegisterTraceFunnelsRoutes(r, am)

	err := s.o11y.APIServer.AddToRouter(r)
	if err != nil {
		return nil, err
	}

	c := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "DELETE", "POST", "PUT", "PATCH", "OPTIONS"},
		AllowedHeaders: []string{"Accept", "Authorization", "Content-Type", "cache-control", "X-HANZO-QUERY-ID", "Sec-WebSocket-Protocol"},
	})

	handler := c.Handler(r)

	handler = handlers.CompressHandler(handler)

	err = web.AddToRouter(r)
	if err != nil {
		return nil, err
	}

	routePrefix := s.config.Global.ExternalPath()
	if routePrefix != "" {
		prefixed := http.StripPrefix(routePrefix, handler)
		handler = http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			switch req.URL.Path {
			case "/v1/o11y/v1/health", "/v1/o11y/v2/healthz", "/v1/o11y/v2/readyz", "/v1/o11y/v2/livez":
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
		return fmt.Errorf("baseconst.HTTPHostPort is required")
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
		slog.Info("Starting OpAmp Websocket server", "addr", baseconst.OpAmpWsEndpoint)
		err := s.opampServer.Start(baseconst.OpAmpWsEndpoint)
		if err != nil {
			slog.Error("opamp ws server failed to start", errors.Attr(err))
			s.unavailableChannel <- healthcheck.Unavailable
		}
	}()

	return nil
}

func (s *Server) Stop(ctx context.Context) error {
	if s.httpServer != nil {
		if err := s.httpServer.Shutdown(ctx); err != nil {
			return err
		}
	}

	s.opampServer.Stop()

	// stop usage manager
	s.usageManager.Stop(ctx)

	return nil
}
