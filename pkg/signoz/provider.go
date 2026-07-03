package signoz

import (
	"github.com/hanzoai/o11y/pkg/alertmanager"
	"github.com/hanzoai/o11y/pkg/alertmanager/nfmanager"
	"github.com/hanzoai/o11y/pkg/alertmanager/nfmanager/rulebasednotification"
	"github.com/hanzoai/o11y/pkg/alertmanager/signozalertmanager"
	"github.com/hanzoai/o11y/pkg/analytics"
	"github.com/hanzoai/o11y/pkg/analytics/noopanalytics"
	"github.com/hanzoai/o11y/pkg/analytics/segmentanalytics"
	"github.com/hanzoai/o11y/pkg/apiserver"
	"github.com/hanzoai/o11y/pkg/apiserver/signozapiserver"
	"github.com/hanzoai/o11y/pkg/auditor"
	"github.com/hanzoai/o11y/pkg/auditor/noopauditor"
	"github.com/hanzoai/o11y/pkg/authz"
	"github.com/hanzoai/o11y/pkg/cache"
	"github.com/hanzoai/o11y/pkg/cache/memorycache"
	"github.com/hanzoai/o11y/pkg/cache/rediscache"
	"github.com/hanzoai/o11y/pkg/emailing"
	"github.com/hanzoai/o11y/pkg/emailing/noopemailing"
	"github.com/hanzoai/o11y/pkg/emailing/smtpemailing"
	"github.com/hanzoai/o11y/pkg/factory"
	"github.com/hanzoai/o11y/pkg/flagger"
	"github.com/hanzoai/o11y/pkg/flagger/configflagger"
	"github.com/hanzoai/o11y/pkg/global"
	"github.com/hanzoai/o11y/pkg/global/signozglobal"
	"github.com/hanzoai/o11y/pkg/identn"
	"github.com/hanzoai/o11y/pkg/identn/apikeyidentn"
	"github.com/hanzoai/o11y/pkg/identn/iamidentn"
	"github.com/hanzoai/o11y/pkg/identn/impersonationidentn"
	"github.com/hanzoai/o11y/pkg/identn/tokenizeridentn"
	"github.com/hanzoai/o11y/pkg/meterreporter"
	"github.com/hanzoai/o11y/pkg/meterreporter/noopmeterreporter"
	"github.com/hanzoai/o11y/pkg/modules/authdomain/implauthdomain"
	"github.com/hanzoai/o11y/pkg/modules/organization"
	"github.com/hanzoai/o11y/pkg/modules/organization/implorganization"
	"github.com/hanzoai/o11y/pkg/modules/preference/implpreference"
	"github.com/hanzoai/o11y/pkg/modules/promote/implpromote"
	"github.com/hanzoai/o11y/pkg/modules/serviceaccount"
	"github.com/hanzoai/o11y/pkg/modules/session/implsession"
	"github.com/hanzoai/o11y/pkg/modules/user"
	"github.com/hanzoai/o11y/pkg/modules/user/impluser"
	"github.com/hanzoai/o11y/pkg/pprof"
	"github.com/hanzoai/o11y/pkg/pprof/httppprof"
	"github.com/hanzoai/o11y/pkg/pprof/nooppprof"
	"github.com/hanzoai/o11y/pkg/prometheus"
	"github.com/hanzoai/o11y/pkg/prometheus/clickhouseprometheus"
	"github.com/hanzoai/o11y/pkg/querier"
	"github.com/hanzoai/o11y/pkg/querier/signozquerier"
	"github.com/hanzoai/o11y/pkg/sharder"
	"github.com/hanzoai/o11y/pkg/sharder/noopsharder"
	"github.com/hanzoai/o11y/pkg/sharder/singlesharder"
	"github.com/hanzoai/o11y/pkg/sqlmigration"
	"github.com/hanzoai/o11y/pkg/sqlschema"
	"github.com/hanzoai/o11y/pkg/sqlschema/sqlitesqlschema"
	"github.com/hanzoai/o11y/pkg/sqlstore"
	"github.com/hanzoai/o11y/pkg/sqlstore/sqlitesqlstore"
	"github.com/hanzoai/o11y/pkg/sqlstore/sqlstorehook"
	"github.com/hanzoai/o11y/pkg/statsreporter"
	"github.com/hanzoai/o11y/pkg/statsreporter/analyticsstatsreporter"
	"github.com/hanzoai/o11y/pkg/statsreporter/noopstatsreporter"
	"github.com/hanzoai/o11y/pkg/telemetrystore"
	"github.com/hanzoai/o11y/pkg/telemetrystore/clickhousetelemetrystore"
	"github.com/hanzoai/o11y/pkg/telemetrystore/telemetrystorehook"
	"github.com/hanzoai/o11y/pkg/tokenizer"
	"github.com/hanzoai/o11y/pkg/tokenizer/jwttokenizer"
	"github.com/hanzoai/o11y/pkg/tokenizer/opaquetokenizer"
	"github.com/hanzoai/o11y/pkg/tokenizer/tokenizerstore/sqltokenizerstore"
	"github.com/hanzoai/o11y/pkg/types/alertmanagertypes"
	"github.com/hanzoai/o11y/pkg/types/featuretypes"
	"github.com/hanzoai/o11y/pkg/version"
	"github.com/hanzoai/o11y/pkg/web"
	"github.com/hanzoai/o11y/pkg/web/noopweb"
	"github.com/hanzoai/o11y/pkg/web/routerweb"
)

func NewAnalyticsProviderFactories() factory.NamedMap[factory.ProviderFactory[analytics.Analytics, analytics.Config]] {
	return factory.MustNewNamedMap(
		noopanalytics.NewFactory(),
		segmentanalytics.NewFactory(),
	)
}

func NewCacheProviderFactories() factory.NamedMap[factory.ProviderFactory[cache.Cache, cache.Config]] {
	return factory.MustNewNamedMap(
		memorycache.NewFactory(),
		rediscache.NewFactory(),
	)
}

func NewWebProviderFactories(globalConfig global.Config) factory.NamedMap[factory.ProviderFactory[web.Web, web.Config]] {
	return factory.MustNewNamedMap(
		routerweb.NewFactory(globalConfig),
		noopweb.NewFactory(),
	)
}

func NewPProfProviderFactories() factory.NamedMap[factory.ProviderFactory[pprof.PProf, pprof.Config]] {
	return factory.MustNewNamedMap(
		httppprof.NewFactory(),
		nooppprof.NewFactory(),
	)
}

func NewSQLStoreProviderFactories() factory.NamedMap[factory.ProviderFactory[sqlstore.SQLStore, sqlstore.Config]] {
	return factory.MustNewNamedMap(
		sqlitesqlstore.NewFactory(sqlstorehook.NewLoggingFactory(), sqlstorehook.NewInstrumentationFactory()),
	)
}

func NewSQLSchemaProviderFactories(sqlstore sqlstore.SQLStore) factory.NamedMap[factory.ProviderFactory[sqlschema.SQLSchema, sqlschema.Config]] {
	return factory.MustNewNamedMap(
		sqlitesqlschema.NewFactory(sqlstore),
	)
}

func NewSQLMigrationProviderFactories(
	sqlstore sqlstore.SQLStore,
	sqlschema sqlschema.SQLSchema,
	telemetryStore telemetrystore.TelemetryStore,
	providerSettings factory.ProviderSettings,
) factory.NamedMap[factory.ProviderFactory[sqlmigration.SQLMigration, sqlmigration.Config]] {
	return factory.MustNewNamedMap(
		sqlmigration.NewAddDataMigrationsFactory(),
		sqlmigration.NewAddOrganizationFactory(),
		sqlmigration.NewAddPreferencesFactory(),
		sqlmigration.NewAddDashboardsFactory(),
		sqlmigration.NewAddSavedViewsFactory(),
		sqlmigration.NewAddAgentsFactory(),
		sqlmigration.NewAddPipelinesFactory(),
		sqlmigration.NewAddIntegrationsFactory(),
		sqlmigration.NewAddLicensesFactory(),
		sqlmigration.NewAddPatsFactory(),
		sqlmigration.NewModifyDatetimeFactory(),
		sqlmigration.NewModifyOrgDomainFactory(),
		sqlmigration.NewUpdateOrganizationFactory(sqlstore),
		sqlmigration.NewAddAlertmanagerFactory(sqlstore),
		sqlmigration.NewUpdateDashboardAndSavedViewsFactory(sqlstore),
		sqlmigration.NewUpdatePatAndOrgDomainsFactory(sqlstore),
		sqlmigration.NewUpdatePipelines(sqlstore),
		sqlmigration.NewDropLicensesSitesFactory(sqlstore),
		sqlmigration.NewUpdateInvitesFactory(sqlstore),
		sqlmigration.NewUpdatePatFactory(sqlstore),
		sqlmigration.NewUpdateAlertmanagerFactory(sqlstore),
		sqlmigration.NewUpdatePreferencesFactory(sqlstore),
		sqlmigration.NewUpdateApdexTtlFactory(sqlstore),
		sqlmigration.NewUpdateResetPasswordFactory(sqlstore),
		sqlmigration.NewUpdateRulesFactory(sqlstore),
		sqlmigration.NewAddVirtualFieldsFactory(),
		sqlmigration.NewUpdateIntegrationsFactory(sqlstore),
		sqlmigration.NewUpdateOrganizationsFactory(sqlstore),
		sqlmigration.NewDropGroupsFactory(sqlstore),
		sqlmigration.NewCreateQuickFiltersFactory(sqlstore),
		sqlmigration.NewUpdateQuickFiltersFactory(sqlstore),
		sqlmigration.NewAuthRefactorFactory(sqlstore),
		sqlmigration.NewUpdateLicenseFactory(sqlstore),
		sqlmigration.NewMigratePATToFactorAPIKey(sqlstore),
		sqlmigration.NewUpdateApiMonitoringFiltersFactory(sqlstore),
		sqlmigration.NewAddKeyOrganizationFactory(sqlstore),
		sqlmigration.NewAddTraceFunnelsFactory(sqlstore),
		sqlmigration.NewUpdateDashboardFactory(sqlstore),
		sqlmigration.NewDropFeatureSetFactory(),
		sqlmigration.NewDropDeprecatedTablesFactory(),
		sqlmigration.NewUpdateAgentsFactory(sqlstore),
		sqlmigration.NewUpdateUsersFactory(sqlstore, sqlschema),
		sqlmigration.NewUpdateUserInviteFactory(sqlstore, sqlschema),
		sqlmigration.NewUpdateOrgDomainFactory(sqlstore, sqlschema),
		sqlmigration.NewAddFactorIndexesFactory(sqlstore, sqlschema),
		sqlmigration.NewQueryBuilderV5MigrationFactory(sqlstore, telemetryStore),
		sqlmigration.NewAddMeterQuickFiltersFactory(sqlstore, sqlschema),
		sqlmigration.NewUpdateTTLSettingForCustomRetentionFactory(sqlstore, sqlschema),
		sqlmigration.NewAddRoutePolicyFactory(sqlstore, sqlschema),
		sqlmigration.NewAddAuthTokenFactory(sqlstore, sqlschema),
		sqlmigration.NewAddAuthzFactory(sqlstore, sqlschema),
		sqlmigration.NewAddPublicDashboardsFactory(sqlstore, sqlschema),
		sqlmigration.NewAddRoleFactory(sqlstore, sqlschema),
		sqlmigration.NewUpdateAuthzFactory(sqlstore, sqlschema),
		sqlmigration.NewUpdateUserPreferenceFactory(sqlstore, sqlschema),
		sqlmigration.NewUpdateOrgPreferenceFactory(sqlstore, sqlschema),
		sqlmigration.NewRenameOrgDomainsFactory(sqlstore, sqlschema),
		sqlmigration.NewAddResetPasswordTokenExpiryFactory(sqlstore, sqlschema),
		sqlmigration.NewAddManagedRolesFactory(sqlstore, sqlschema),
		sqlmigration.NewAddAuthzIndexFactory(sqlstore, sqlschema),
		sqlmigration.NewMigrateRbacToAuthzFactory(sqlstore),
		sqlmigration.NewMigratePublicDashboardsFactory(sqlstore),
		sqlmigration.NewAddAnonymousPublicDashboardTransactionFactory(sqlstore),
		sqlmigration.NewAddRootUserFactory(sqlstore, sqlschema),
		sqlmigration.NewAddUserEmailOrgIDIndexFactory(sqlstore, sqlschema),
		sqlmigration.NewMigrateRulesV4ToV5Factory(sqlstore, telemetryStore),
		sqlmigration.NewAddStatusUserFactory(sqlstore, sqlschema),
		sqlmigration.NewDeprecateUserInviteFactory(sqlstore, sqlschema),
		sqlmigration.NewUpdateCloudIntegrationUniqueIndexFactory(sqlstore, sqlschema),
		sqlmigration.NewUpdatePlannedMaintenanceRuleFactory(sqlstore, sqlschema),
		sqlmigration.NewAddUserRoleFactory(sqlstore, sqlschema),
		sqlmigration.NewDropUserRoleColumnFactory(sqlstore, sqlschema),
		sqlmigration.NewAddServiceAccountFactory(sqlstore, sqlschema),
		sqlmigration.NewDeprecateAPIKeyFactory(sqlstore, sqlschema),
		sqlmigration.NewServiceAccountAuthzactory(sqlstore),
		sqlmigration.NewDropUserDeletedAtFactory(sqlstore, sqlschema),
		sqlmigration.NewMigrateAWSAllRegionsFactory(sqlstore),
		sqlmigration.NewAddServiceAccountManagedRoleTransactionsFactory(sqlstore),
		sqlmigration.NewAddSpanMapperFactory(sqlstore, sqlschema),
		sqlmigration.NewAddLLMPricingRulesFactory(sqlstore, sqlschema),
		sqlmigration.NewMigrateMetaresourcesTuplesFactory(sqlstore),
		sqlmigration.NewAddTagsFactory(sqlstore, sqlschema),
		sqlmigration.NewAddRoleCRUDTuplesFactory(sqlstore),
		sqlmigration.NewAddIntegrationDashboardFactory(sqlstore, sqlschema),
		sqlmigration.NewAddSourceToDashboardFactory(sqlstore, sqlschema),
		sqlmigration.NewMigrateCloudIntegrationDashboardsFactory(sqlstore),
		sqlmigration.NewAddScopeToPlannedMaintenanceFactory(sqlstore, sqlschema),
		sqlmigration.NewMigrateInstalledIntegrationDashboardsFactory(sqlstore),
		sqlmigration.NewAddDashboardNameFactory(sqlstore, sqlschema),
		sqlmigration.NewFixChangelogOperationTypeFactory(sqlstore, sqlschema),
		sqlmigration.NewCloudIntegrationRemoveCascadeDeleteFactory(sqlschema),
		sqlmigration.NewAddUserDashboardPreferenceFactory(sqlstore, sqlschema),
		sqlmigration.NewRecreateUserDashboardPreferenceFactory(sqlstore, sqlschema),
		sqlmigration.NewMigrateRecurrenceBoundsFactory(sqlstore),
		sqlmigration.NewAddDashboardViewFactory(sqlstore, sqlschema),
		sqlmigration.NewMigrateSSORoleMappingNamesFactory(sqlstore),
		sqlmigration.NewAddMetricReductionRulesFactory(sqlstore, sqlschema),
		sqlmigration.NewRemoveOrganizationTuplesFactory(sqlstore),
		sqlmigration.NewAddLLMObsFactory(sqlstore, sqlschema),
	)
}

func NewTelemetryStoreProviderFactories() factory.NamedMap[factory.ProviderFactory[telemetrystore.TelemetryStore, telemetrystore.Config]] {
	return factory.MustNewNamedMap(
		clickhousetelemetrystore.NewFactory(
			telemetrystorehook.NewLoggingFactory(),
			// adding instrumentation factory before settings as we are starting the query span here
			telemetrystorehook.NewInstrumentationFactory(),
			telemetrystorehook.NewSettingsFactory(),
		),
	)
}

func NewPrometheusProviderFactories(telemetryStore telemetrystore.TelemetryStore) factory.NamedMap[factory.ProviderFactory[prometheus.Prometheus, prometheus.Config]] {
	return factory.MustNewNamedMap(
		clickhouseprometheus.NewFactory(telemetryStore),
	)
}

func NewNotificationManagerProviderFactories(routeStore alertmanagertypes.RouteStore) factory.NamedMap[factory.ProviderFactory[nfmanager.NotificationManager, nfmanager.Config]] {
	return factory.MustNewNamedMap(
		rulebasednotification.NewFactory(routeStore),
	)
}

func NewAlertmanagerProviderFactories(
	sqlstore sqlstore.SQLStore,
	orgGetter organization.Getter,
	nfManager nfmanager.NotificationManager,
	maintenanceStore alertmanagertypes.MaintenanceStore,
) factory.NamedMap[factory.ProviderFactory[alertmanager.Alertmanager, alertmanager.Config]] {
	return factory.MustNewNamedMap(
		signozalertmanager.NewFactory(sqlstore, orgGetter, nfManager, maintenanceStore),
	)
}

func NewEmailingProviderFactories() factory.NamedMap[factory.ProviderFactory[emailing.Emailing, emailing.Config]] {
	return factory.MustNewNamedMap(
		noopemailing.NewFactory(),
		smtpemailing.NewFactory(),
	)
}

func NewSharderProviderFactories() factory.NamedMap[factory.ProviderFactory[sharder.Sharder, sharder.Config]] {
	return factory.MustNewNamedMap(
		singlesharder.NewFactory(),
		noopsharder.NewFactory(),
	)
}

func NewStatsReporterProviderFactories(aggregator statsreporter.Aggregator, orgGetter organization.Getter, userGetter user.Getter, tokenizer tokenizer.Tokenizer, build version.Build, analyticsConfig analytics.Config) factory.NamedMap[factory.ProviderFactory[statsreporter.StatsReporter, statsreporter.Config]] {
	return factory.MustNewNamedMap(
		analyticsstatsreporter.NewFactory(aggregator, orgGetter, userGetter, tokenizer, build, analyticsConfig),
		noopstatsreporter.NewFactory(),
	)
}

func NewQuerierProviderFactories(telemetryStore telemetrystore.TelemetryStore, prometheus prometheus.Prometheus, cache cache.Cache, flagger flagger.Flagger) factory.NamedMap[factory.ProviderFactory[querier.Querier, querier.Config]] {
	return factory.MustNewNamedMap(
		signozquerier.NewFactory(telemetryStore, prometheus, cache, flagger),
	)
}

func NewAPIServerProviderFactories(orgGetter organization.Getter, authz authz.AuthZ, modules Modules, handlers Handlers, globalConfig global.Config) factory.NamedMap[factory.ProviderFactory[apiserver.APIServer, apiserver.Config]] {
	return factory.MustNewNamedMap(
		signozapiserver.NewFactory(
			orgGetter,
			authz,
			implorganization.NewHandler(modules.OrgGetter, modules.OrgSetter),
			impluser.NewHandler(modules.UserSetter, modules.UserGetter),
			implsession.NewHandler(modules.Session, globalConfig),
			implauthdomain.NewHandler(modules.AuthDomain),
			implpreference.NewHandler(modules.Preference),
			handlers.Global,
			implpromote.NewHandler(modules.Promote),
			handlers.FlaggerHandler,
			modules.Dashboard,
			handlers.Dashboard,
			handlers.MetricsExplorer,
			handlers.MetricReductionRule,
			handlers.InfraMonitoring,
			handlers.GatewayHandler,
			handlers.Fields,
			handlers.AuthzHandler,
			handlers.RawDataExport,
			handlers.ZeusHandler,
			handlers.QuerierHandler,
			handlers.ServiceAccountHandler,
			handlers.RegistryHandler,
			handlers.CloudIntegrationHandler,
			handlers.RuleStateHistory,
			handlers.SpanMapperHandler,
			handlers.AlertmanagerHandler,
			handlers.LLMPricingRuleHandler,
			handlers.TraceDetail,
			handlers.RulerHandler,
			handlers.StatsHandler,
			handlers.LLMObsHandler,
		),
	)
}

func NewTokenizerProviderFactories(cache cache.Cache, sqlstore sqlstore.SQLStore, orgGetter organization.Getter) factory.NamedMap[factory.ProviderFactory[tokenizer.Tokenizer, tokenizer.Config]] {
	tokenStore := sqltokenizerstore.NewStore(sqlstore)
	return factory.MustNewNamedMap(
		opaquetokenizer.NewFactory(cache, tokenStore, orgGetter),
		jwttokenizer.NewFactory(cache, tokenStore),
	)
}

func NewIdentNProviderFactories(tokenizer tokenizer.Tokenizer, serviceAccount serviceaccount.Module, orgGetter organization.Getter, orgSetter organization.Setter, authz authz.AuthZ, userGetter user.Getter, userConfig user.Config) factory.NamedMap[factory.ProviderFactory[identn.IdentN, identn.Config]] {
	return factory.MustNewNamedMap(
		iamidentn.NewFactory(orgGetter, orgSetter, authz),
		impersonationidentn.NewFactory(orgGetter, userGetter, userConfig),
		tokenizeridentn.NewFactory(tokenizer),
		apikeyidentn.NewFactory(serviceAccount),
	)
}

func NewGlobalProviderFactories(identNConfig identn.Config) factory.NamedMap[factory.ProviderFactory[global.Global, global.Config]] {
	return factory.MustNewNamedMap(
		signozglobal.NewFactory(identNConfig),
	)
}

func NewAuditorProviderFactories() factory.NamedMap[factory.ProviderFactory[auditor.Auditor, auditor.Config]] {
	return factory.MustNewNamedMap(
		noopauditor.NewFactory(),
	)
}

func NewMeterReporterProviderFactories() factory.NamedMap[factory.ProviderFactory[meterreporter.Reporter, meterreporter.Config]] {
	return factory.MustNewNamedMap(
		noopmeterreporter.NewFactory(),
	)
}

func NewFlaggerProviderFactories(registry featuretypes.Registry) factory.NamedMap[factory.ProviderFactory[flagger.FlaggerProvider, flagger.Config]] {
	return factory.MustNewNamedMap(
		configflagger.NewFactory(registry),
	)
}
