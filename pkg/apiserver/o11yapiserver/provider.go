package o11yapiserver

import (
	"context"

	"github.com/hanzoai/o11y/pkg/alertmanager"
	"github.com/hanzoai/o11y/pkg/apiserver"
	"github.com/hanzoai/o11y/pkg/authz"
	"github.com/hanzoai/o11y/pkg/factory"
	"github.com/hanzoai/o11y/pkg/flagger"
	"github.com/hanzoai/o11y/pkg/gateway"
	"github.com/hanzoai/o11y/pkg/global"
	"github.com/hanzoai/o11y/pkg/http/handler"
	"github.com/hanzoai/o11y/pkg/http/middleware"
	"github.com/hanzoai/o11y/pkg/http/routing"
	"github.com/hanzoai/o11y/pkg/modules/authdomain"
	"github.com/hanzoai/o11y/pkg/modules/cloudintegration"
	"github.com/hanzoai/o11y/pkg/modules/dashboard"
	"github.com/hanzoai/o11y/pkg/modules/errortracking"
	"github.com/hanzoai/o11y/pkg/modules/fields"
	"github.com/hanzoai/o11y/pkg/modules/inframonitoring"
	"github.com/hanzoai/o11y/pkg/modules/llmobs"
	"github.com/hanzoai/o11y/pkg/modules/llmpricingrule"
	"github.com/hanzoai/o11y/pkg/modules/metricreductionrule"
	"github.com/hanzoai/o11y/pkg/modules/metricsexplorer"
	"github.com/hanzoai/o11y/pkg/modules/organization"
	"github.com/hanzoai/o11y/pkg/modules/preference"
	"github.com/hanzoai/o11y/pkg/modules/promote"
	"github.com/hanzoai/o11y/pkg/modules/rawdataexport"
	"github.com/hanzoai/o11y/pkg/modules/rulestatehistory"
	"github.com/hanzoai/o11y/pkg/modules/sentry"
	"github.com/hanzoai/o11y/pkg/modules/serviceaccount"
	"github.com/hanzoai/o11y/pkg/modules/session"
	"github.com/hanzoai/o11y/pkg/modules/spanmapper"
	"github.com/hanzoai/o11y/pkg/modules/tracedetail"
	"github.com/hanzoai/o11y/pkg/modules/user"
	"github.com/hanzoai/o11y/pkg/querier"
	"github.com/hanzoai/o11y/pkg/ruler"
	"github.com/hanzoai/o11y/pkg/statsreporter"
	"github.com/hanzoai/o11y/pkg/types"
	"github.com/hanzoai/o11y/pkg/types/authtypes"
	"github.com/hanzoai/o11y/pkg/zeus"
)

type provider struct {
	config                     apiserver.Config
	settings                   factory.ScopedProviderSettings
	authzMiddleware            *middleware.AuthZ
	authzService               authz.AuthZ
	orgHandler                 organization.Handler
	userHandler                user.Handler
	sessionHandler             session.Handler
	authDomainHandler          authdomain.Handler
	preferenceHandler          preference.Handler
	globalHandler              global.Handler
	promoteHandler             promote.Handler
	flaggerHandler             flagger.Handler
	dashboardModule            dashboard.Module
	dashboardHandler           dashboard.Handler
	metricsExplorerHandler     metricsexplorer.Handler
	metricReductionRuleHandler metricreductionrule.Handler
	infraMonitoringHandler     inframonitoring.Handler
	gatewayHandler             gateway.Handler
	fieldsHandler              fields.Handler
	authzHandler               authz.Handler
	rawDataExportHandler       rawdataexport.Handler
	zeusHandler                zeus.Handler
	querierHandler             querier.Handler
	serviceAccountHandler      serviceaccount.Handler
	factoryHandler             factory.Handler
	cloudIntegrationHandler    cloudintegration.Handler
	ruleStateHistoryHandler    rulestatehistory.Handler
	spanMapperHandler          spanmapper.Handler
	alertmanagerHandler        alertmanager.Handler
	traceDetailHandler         tracedetail.Handler
	rulerHandler               ruler.Handler
	llmPricingRuleHandler      llmpricingrule.Handler
	llmObsHandler              llmobs.Handler
	errorTrackingHandler       errortracking.Handler
	sentryHandler              sentry.Handler
	statsHandler               statsreporter.Handler
}

func NewFactory(
	orgGetter organization.Getter,
	authzService authz.AuthZ,
	orgHandler organization.Handler,
	userHandler user.Handler,
	sessionHandler session.Handler,
	authDomainHandler authdomain.Handler,
	preferenceHandler preference.Handler,
	globalHandler global.Handler,
	promoteHandler promote.Handler,
	flaggerHandler flagger.Handler,
	dashboardModule dashboard.Module,
	dashboardHandler dashboard.Handler,
	metricsExplorerHandler metricsexplorer.Handler,
	metricReductionRuleHandler metricreductionrule.Handler,
	infraMonitoringHandler inframonitoring.Handler,
	gatewayHandler gateway.Handler,
	fieldsHandler fields.Handler,
	authzHandler authz.Handler,
	rawDataExportHandler rawdataexport.Handler,
	zeusHandler zeus.Handler,
	querierHandler querier.Handler,
	serviceAccountHandler serviceaccount.Handler,
	factoryHandler factory.Handler,
	cloudIntegrationHandler cloudintegration.Handler,
	ruleStateHistoryHandler rulestatehistory.Handler,
	spanMapperHandler spanmapper.Handler,
	alertmanagerHandler alertmanager.Handler,
	llmPricingRuleHandler llmpricingrule.Handler,
	traceDetailHandler tracedetail.Handler,
	rulerHandler ruler.Handler,
	statsHandler statsreporter.Handler,
	llmObsHandler llmobs.Handler,
	errorTrackingHandler errortracking.Handler,
	sentryHandler sentry.Handler,
) factory.ProviderFactory[apiserver.APIServer, apiserver.Config] {
	return factory.NewProviderFactory(factory.MustNewName("o11y"), func(ctx context.Context, providerSettings factory.ProviderSettings, config apiserver.Config) (apiserver.APIServer, error) {
		return newProvider(
			ctx,
			providerSettings,
			config,
			orgGetter,
			authzService,
			orgHandler,
			userHandler,
			sessionHandler,
			authDomainHandler,
			preferenceHandler,
			globalHandler,
			promoteHandler,
			flaggerHandler,
			dashboardModule,
			dashboardHandler,
			metricsExplorerHandler,
			metricReductionRuleHandler,
			infraMonitoringHandler,
			gatewayHandler,
			fieldsHandler,
			authzHandler,
			rawDataExportHandler,
			zeusHandler,
			querierHandler,
			serviceAccountHandler,
			factoryHandler,
			cloudIntegrationHandler,
			ruleStateHistoryHandler,
			spanMapperHandler,
			alertmanagerHandler,
			llmPricingRuleHandler,
			traceDetailHandler,
			rulerHandler,
			statsHandler,
			llmObsHandler,
			errorTrackingHandler,
			sentryHandler,
		)
	})
}

func newProvider(
	_ context.Context,
	providerSettings factory.ProviderSettings,
	config apiserver.Config,
	orgGetter organization.Getter,
	authzService authz.AuthZ,
	orgHandler organization.Handler,
	userHandler user.Handler,
	sessionHandler session.Handler,
	authDomainHandler authdomain.Handler,
	preferenceHandler preference.Handler,
	globalHandler global.Handler,
	promoteHandler promote.Handler,
	flaggerHandler flagger.Handler,
	dashboardModule dashboard.Module,
	dashboardHandler dashboard.Handler,
	metricsExplorerHandler metricsexplorer.Handler,
	metricReductionRuleHandler metricreductionrule.Handler,
	infraMonitoringHandler inframonitoring.Handler,
	gatewayHandler gateway.Handler,
	fieldsHandler fields.Handler,
	authzHandler authz.Handler,
	rawDataExportHandler rawdataexport.Handler,
	zeusHandler zeus.Handler,
	querierHandler querier.Handler,
	serviceAccountHandler serviceaccount.Handler,
	factoryHandler factory.Handler,
	cloudIntegrationHandler cloudintegration.Handler,
	ruleStateHistoryHandler rulestatehistory.Handler,
	spanMapperHandler spanmapper.Handler,
	alertmanagerHandler alertmanager.Handler,
	llmPricingRuleHandler llmpricingrule.Handler,
	traceDetailHandler tracedetail.Handler,
	rulerHandler ruler.Handler,
	statsHandler statsreporter.Handler,
	llmObsHandler llmobs.Handler,
	errorTrackingHandler errortracking.Handler,
	sentryHandler sentry.Handler,
) (apiserver.APIServer, error) {
	settings := factory.NewScopedProviderSettings(providerSettings, "github.com/hanzoai/o11y/pkg/apiserver/o11yapiserver")
	provider := &provider{
		config:                     config,
		settings:                   settings,
		orgHandler:                 orgHandler,
		userHandler:                userHandler,
		authzService:               authzService,
		sessionHandler:             sessionHandler,
		authDomainHandler:          authDomainHandler,
		preferenceHandler:          preferenceHandler,
		globalHandler:              globalHandler,
		promoteHandler:             promoteHandler,
		flaggerHandler:             flaggerHandler,
		dashboardModule:            dashboardModule,
		dashboardHandler:           dashboardHandler,
		metricsExplorerHandler:     metricsExplorerHandler,
		metricReductionRuleHandler: metricReductionRuleHandler,
		infraMonitoringHandler:     infraMonitoringHandler,
		gatewayHandler:             gatewayHandler,
		fieldsHandler:              fieldsHandler,
		authzHandler:               authzHandler,
		rawDataExportHandler:       rawDataExportHandler,
		zeusHandler:                zeusHandler,
		querierHandler:             querierHandler,
		serviceAccountHandler:      serviceAccountHandler,
		factoryHandler:             factoryHandler,
		cloudIntegrationHandler:    cloudIntegrationHandler,
		ruleStateHistoryHandler:    ruleStateHistoryHandler,
		spanMapperHandler:          spanMapperHandler,
		alertmanagerHandler:        alertmanagerHandler,
		traceDetailHandler:         traceDetailHandler,
		rulerHandler:               rulerHandler,
		llmPricingRuleHandler:      llmPricingRuleHandler,
		llmObsHandler:              llmObsHandler,
		errorTrackingHandler:       errorTrackingHandler,
		sentryHandler:              sentryHandler,
		statsHandler:               statsHandler,
	}

	provider.authzMiddleware = middleware.NewAuthZ(settings.Logger(), orgGetter, authzService)

	// No router of its own. The provider used to build one, register every route
	// on it, and answer Router() with it — a second copy of the whole tree that
	// nothing served from and nothing read. The host's router is the only one.
	return provider, nil
}

func (provider *provider) AddToRouter(router routing.Router) {
	provider.addOrgRoutes(router)
	provider.addSessionRoutes(router)
	provider.addAuthDomainRoutes(router)
	provider.addPreferenceRoutes(router)
	provider.addUserRoutes(router)
	provider.addGlobalRoutes(router)
	provider.addPromoteRoutes(router)
	provider.addFlaggerRoutes(router)
	provider.addDashboardRoutes(router)
	provider.addMetricsExplorerRoutes(router)
	provider.addMetricReductionRuleRoutes(router)
	provider.addInfraMonitoringRoutes(router)
	provider.addGatewayRoutes(router)
	provider.addRoleRoutes(router)
	provider.addAuthzRoutes(router)
	provider.addFieldsRoutes(router)
	provider.addRawDataExportRoutes(router)
	provider.addZeusRoutes(router)
	provider.addQuerierRoutes(router)
	provider.addServiceAccountRoutes(router)
	provider.addRegistryRoutes(router)
	provider.addCloudIntegrationRoutes(router)
	provider.addRuleStateHistoryRoutes(router)
	provider.addSpanMapperRoutes(router)
	provider.addAlertmanagerRoutes(router)
	provider.addLLMPricingRuleRoutes(router)
	provider.addLLMObsRoutes(router)
	provider.addErrorTrackingRoutes(router)
	// Hanzo Sentry product face — clean /v1/sentinel/* routes on THIS router.
	// Registered here so its literal paths precede the ingest wildcards.
	provider.addSentryRoutes(router)
	provider.addTraceDetailRoutes(router)
	provider.addRulerRoutes(router)
	provider.addStatsReporterRoutes(router)
}

func newSecuritySchemes(role types.Role) []handler.OpenAPISecurityScheme {
	return newScopedSecuritySchemes([]string{role.String()})
}

func newAnonymousSecuritySchemes(scopes []string) []handler.OpenAPISecurityScheme {
	return []handler.OpenAPISecurityScheme{
		{Name: authtypes.IdentNProviderAnonymous.StringValue(), Scopes: scopes},
	}
}

func newScopedSecuritySchemes(scopes []string) []handler.OpenAPISecurityScheme {
	return []handler.OpenAPISecurityScheme{
		{Name: authtypes.IdentNProviderAPIKey.StringValue(), Scopes: scopes},
		{Name: authtypes.IdentNProviderTokenizer.StringValue(), Scopes: scopes},
	}
}
