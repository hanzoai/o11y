package o11yapiserver

import (
	"context"

	"github.com/gorilla/mux"
	"github.com/hanzoai/o11y/pkg/alertmanager"
	"github.com/hanzoai/o11y/pkg/apiserver"
	"github.com/hanzoai/o11y/pkg/authz"
	"github.com/hanzoai/o11y/pkg/factory"
	"github.com/hanzoai/o11y/pkg/flagger"
	"github.com/hanzoai/o11y/pkg/gateway"
	"github.com/hanzoai/o11y/pkg/global"
	"github.com/hanzoai/o11y/pkg/http/handler"
	"github.com/hanzoai/o11y/pkg/http/middleware"
	"github.com/hanzoai/o11y/pkg/modules/authdomain"
	"github.com/hanzoai/o11y/pkg/modules/cloudintegration"
	"github.com/hanzoai/o11y/pkg/modules/dashboard"
	"github.com/hanzoai/o11y/pkg/modules/fields"
	"github.com/hanzoai/o11y/pkg/modules/inframonitoring"
	"github.com/hanzoai/o11y/pkg/modules/errortracking"
	"github.com/hanzoai/o11y/pkg/modules/llmobs"
	"github.com/hanzoai/o11y/pkg/modules/llmpricingrule"
	"github.com/hanzoai/o11y/pkg/modules/metricreductionrule"
	"github.com/hanzoai/o11y/pkg/modules/metricsexplorer"
	"github.com/hanzoai/o11y/pkg/modules/organization"
	"github.com/hanzoai/o11y/pkg/modules/preference"
	"github.com/hanzoai/o11y/pkg/modules/promote"
	"github.com/hanzoai/o11y/pkg/modules/rawdataexport"
	"github.com/hanzoai/o11y/pkg/modules/rulestatehistory"
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
	router                     *mux.Router
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
) (apiserver.APIServer, error) {
	settings := factory.NewScopedProviderSettings(providerSettings, "github.com/hanzoai/o11y/pkg/apiserver/o11yapiserver")
	router := mux.NewRouter().UseEncodedPath()

	provider := &provider{
		config:                     config,
		settings:                   settings,
		router:                     router,
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
		statsHandler:               statsHandler,
	}

	provider.authzMiddleware = middleware.NewAuthZ(settings.Logger(), orgGetter, authzService)

	if err := provider.AddToRouter(router); err != nil {
		return nil, err
	}

	return provider, nil
}

func (provider *provider) Router() *mux.Router {
	return provider.router
}

func (provider *provider) AddToRouter(router *mux.Router) error {
	if err := provider.addOrgRoutes(router); err != nil {
		return err
	}

	if err := provider.addSessionRoutes(router); err != nil {
		return err
	}

	if err := provider.addAuthDomainRoutes(router); err != nil {
		return err
	}

	if err := provider.addPreferenceRoutes(router); err != nil {
		return err
	}

	if err := provider.addUserRoutes(router); err != nil {
		return err
	}

	if err := provider.addGlobalRoutes(router); err != nil {
		return err
	}

	if err := provider.addPromoteRoutes(router); err != nil {
		return err
	}

	if err := provider.addFlaggerRoutes(router); err != nil {
		return err
	}

	if err := provider.addDashboardRoutes(router); err != nil {
		return err
	}

	if err := provider.addMetricsExplorerRoutes(router); err != nil {
		return err
	}

	if err := provider.addMetricReductionRuleRoutes(router); err != nil {
		return err
	}

	if err := provider.addInfraMonitoringRoutes(router); err != nil {
		return err
	}

	if err := provider.addGatewayRoutes(router); err != nil {
		return err
	}

	if err := provider.addRoleRoutes(router); err != nil {
		return err
	}

	if err := provider.addAuthzRoutes(router); err != nil {
		return err
	}

	if err := provider.addFieldsRoutes(router); err != nil {
		return err
	}

	if err := provider.addRawDataExportRoutes(router); err != nil {
		return err
	}

	if err := provider.addZeusRoutes(router); err != nil {
		return err
	}

	if err := provider.addQuerierRoutes(router); err != nil {
		return err
	}

	if err := provider.addServiceAccountRoutes(router); err != nil {
		return err
	}

	if err := provider.addRegistryRoutes(router); err != nil {
		return err
	}

	if err := provider.addCloudIntegrationRoutes(router); err != nil {
		return err
	}

	if err := provider.addRuleStateHistoryRoutes(router); err != nil {
		return err
	}

	if err := provider.addSpanMapperRoutes(router); err != nil {
		return err
	}

	if err := provider.addAlertmanagerRoutes(router); err != nil {
		return err
	}

	if err := provider.addLLMPricingRuleRoutes(router); err != nil {
		return err
	}

	if err := provider.addLLMObsRoutes(router); err != nil {
		return err
	}

	if err := provider.addErrorTrackingRoutes(router); err != nil {
		return err
	}

	if err := provider.addTraceDetailRoutes(router); err != nil {
		return err
	}

	if err := provider.addRulerRoutes(router); err != nil {
		return err
	}

	if err := provider.addStatsReporterRoutes(router); err != nil {
		return err
	}

	// LAST: every /api/vN/* route is now on the router. Register version-less
	// /api/<resource> aliases so the Hanzo public contract /v1/o11y/<resource> never
	// carries O11y's engine version (highest version wins; llmobs names are left
	// owned). Generated by walking the router — no hand map, survives re-syncs.
	if err := AddVersionlessAliases(router); err != nil {
		return err
	}

	return nil
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
