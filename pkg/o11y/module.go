package o11y

import (
	"github.com/hanzoai/o11y/pkg/alertmanager"
	"github.com/hanzoai/o11y/pkg/analytics"
	"github.com/hanzoai/o11y/pkg/authn"
	"github.com/hanzoai/o11y/pkg/authz"
	"github.com/hanzoai/o11y/pkg/cache"
	"github.com/hanzoai/o11y/pkg/emailing"
	"github.com/hanzoai/o11y/pkg/factory"
	"github.com/hanzoai/o11y/pkg/flagger"
	"github.com/hanzoai/o11y/pkg/modules/apdex"
	"github.com/hanzoai/o11y/pkg/modules/apdex/implapdex"
	"github.com/hanzoai/o11y/pkg/modules/authdomain"
	"github.com/hanzoai/o11y/pkg/modules/authdomain/implauthdomain"
	"github.com/hanzoai/o11y/pkg/modules/cloudintegration"
	"github.com/hanzoai/o11y/pkg/modules/dashboard"
	"github.com/hanzoai/o11y/pkg/modules/errortracking"
	"github.com/hanzoai/o11y/pkg/modules/errortracking/implerrortracking"
	"github.com/hanzoai/o11y/pkg/modules/inframonitoring"
	"github.com/hanzoai/o11y/pkg/modules/inframonitoring/implinframonitoring"
	"github.com/hanzoai/o11y/pkg/modules/llmobs"
	"github.com/hanzoai/o11y/pkg/modules/llmobs/impllmobs"
	"github.com/hanzoai/o11y/pkg/modules/llmpricingrule"
	"github.com/hanzoai/o11y/pkg/modules/llmpricingrule/impllmpricingrule"
	"github.com/hanzoai/o11y/pkg/modules/logspipeline"
	"github.com/hanzoai/o11y/pkg/modules/logspipeline/impllogspipeline"
	"github.com/hanzoai/o11y/pkg/modules/metricreductionrule"
	"github.com/hanzoai/o11y/pkg/modules/metricsexplorer"
	"github.com/hanzoai/o11y/pkg/modules/metricsexplorer/implmetricsexplorer"
	"github.com/hanzoai/o11y/pkg/modules/organization"
	"github.com/hanzoai/o11y/pkg/modules/organization/implorganization"
	"github.com/hanzoai/o11y/pkg/modules/preference"
	"github.com/hanzoai/o11y/pkg/modules/preference/implpreference"
	"github.com/hanzoai/o11y/pkg/modules/promote"
	"github.com/hanzoai/o11y/pkg/modules/promote/implpromote"
	"github.com/hanzoai/o11y/pkg/modules/quickfilter"
	"github.com/hanzoai/o11y/pkg/modules/quickfilter/implquickfilter"
	"github.com/hanzoai/o11y/pkg/modules/rawdataexport"
	"github.com/hanzoai/o11y/pkg/modules/rawdataexport/implrawdataexport"
	"github.com/hanzoai/o11y/pkg/modules/retention"
	"github.com/hanzoai/o11y/pkg/modules/rulestatehistory"
	"github.com/hanzoai/o11y/pkg/modules/rulestatehistory/implrulestatehistory"
	"github.com/hanzoai/o11y/pkg/modules/savedview"
	"github.com/hanzoai/o11y/pkg/modules/savedview/implsavedview"
	"github.com/hanzoai/o11y/pkg/modules/sentry"
	"github.com/hanzoai/o11y/pkg/modules/sentry/implsentry"
	"github.com/hanzoai/o11y/pkg/modules/serviceaccount"
	"github.com/hanzoai/o11y/pkg/modules/services"
	"github.com/hanzoai/o11y/pkg/modules/services/implservices"
	"github.com/hanzoai/o11y/pkg/modules/session"
	"github.com/hanzoai/o11y/pkg/modules/session/implsession"
	"github.com/hanzoai/o11y/pkg/modules/spanmapper"
	"github.com/hanzoai/o11y/pkg/modules/spanmapper/implspanmapper"
	"github.com/hanzoai/o11y/pkg/modules/spanpercentile"
	"github.com/hanzoai/o11y/pkg/modules/spanpercentile/implspanpercentile"
	"github.com/hanzoai/o11y/pkg/modules/tag"
	"github.com/hanzoai/o11y/pkg/modules/tracedetail"
	"github.com/hanzoai/o11y/pkg/modules/tracedetail/impltracedetail"
	"github.com/hanzoai/o11y/pkg/modules/tracefunnel"
	"github.com/hanzoai/o11y/pkg/modules/tracefunnel/impltracefunnel"
	"github.com/hanzoai/o11y/pkg/modules/user"
	"github.com/hanzoai/o11y/pkg/modules/user/impluser"
	"github.com/hanzoai/o11y/pkg/querier"
	"github.com/hanzoai/o11y/pkg/queryparser"
	"github.com/hanzoai/o11y/pkg/ruler/rulestore/sqlrulestore"
	"github.com/hanzoai/o11y/pkg/sqlstore"
	"github.com/hanzoai/o11y/pkg/telemetrystore"
	"github.com/hanzoai/o11y/pkg/tokenizer"
	"github.com/hanzoai/o11y/pkg/types/authtypes"
	"github.com/hanzoai/o11y/pkg/types/preferencetypes"
	"github.com/hanzoai/o11y/pkg/types/telemetrytypes"
)

type Modules struct {
	OrgGetter           organization.Getter
	OrgSetter           organization.Setter
	Preference          preference.Module
	UserSetter          user.Setter
	UserGetter          user.Getter
	RetentionGetter     retention.Getter
	SavedView           savedview.Module
	Apdex               apdex.Module
	Dashboard           dashboard.Module
	QuickFilter         quickfilter.Module
	TraceFunnel         tracefunnel.Module
	RawDataExport       rawdataexport.Module
	AuthDomain          authdomain.Module
	Session             session.Module
	Services            services.Module
	SpanPercentile      spanpercentile.Module
	MetricsExplorer     metricsexplorer.Module
	MetricReductionRule metricreductionrule.Module
	InfraMonitoring     inframonitoring.Module
	Promote             promote.Module
	ServiceAccount      serviceaccount.Module
	CloudIntegration    cloudintegration.Module
	LogsPipeline        logspipeline.Module
	RuleStateHistory    rulestatehistory.Module
	TraceDetail         tracedetail.Module
	SpanMapper          spanmapper.Module
	LLMPricingRule      llmpricingrule.Module
	LLMObs              llmobs.Module
	ErrorTracking       errortracking.Module
	// ErrorTrackingRevocations backs per-org DSN-key rotation; the handler consults
	// it on every ingest. Built here because it needs the sqlstore.
	ErrorTrackingRevocations implerrortracking.RevocationStore
	// Sentry is the /v1/sentry product face: it COMPOSES the reused errortracking
	// engine + issue lifecycle, the columnar events plane (telemetryStore) and the
	// reused tracedetail read. Built here because it needs BOTH the sqlstore (projects)
	// and the telemetryStore (events plane).
	Sentry sentry.Module
	Tag    tag.Module
}

func NewModules(
	sqlstore sqlstore.SQLStore,
	tokenizer tokenizer.Tokenizer,
	emailing emailing.Emailing,
	providerSettings factory.ProviderSettings,
	orgGetter organization.Getter,
	alertmanager alertmanager.Alertmanager,
	analytics analytics.Analytics,
	querier querier.Querier,
	telemetryStore telemetrystore.TelemetryStore,
	telemetryMetadataStore telemetrytypes.MetadataStore,
	authNs map[authtypes.AuthNProvider]authn.AuthN,
	authz authz.AuthZ,
	cache cache.Cache,
	queryParser queryparser.QueryParser,
	config Config,
	dashboard dashboard.Module,
	userGetter user.Getter,
	userRoleStore authtypes.UserRoleStore,
	serviceAccount serviceaccount.Module,
	cloudIntegrationModule cloudintegration.Module,
	retentionGetter retention.Getter,
	fl flagger.Flagger,
	tagModule tag.Module,
	metricReductionRule metricreductionrule.Module,
) Modules {
	quickfilter := implquickfilter.NewModule(implquickfilter.NewStore(sqlstore))
	orgSetter := implorganization.NewSetter(implorganization.NewStore(sqlstore), alertmanager, quickfilter)
	// Cleanup callbacks from other modules, invoked when a user is deleted.
	onDeleteUser := []user.OnDeleteUser{
		dashboard.DeletePreferencesForUser,
	}
	userSetter := impluser.NewSetter(impluser.NewStore(sqlstore, providerSettings), tokenizer, emailing, providerSettings, orgSetter, authz, analytics, config.User, userRoleStore, userGetter, onDeleteUser)
	ruleStore := sqlrulestore.NewRuleStore(sqlstore, queryParser, providerSettings)
	authDomainModule := implauthdomain.NewModule(implauthdomain.NewStore(sqlstore), authNs, authz)

	// Error tracking (o11y_issues lifecycle) and trace detail (o11y_traces waterfall)
	// are pulled into locals so the Sentry product face can COMPOSE them rather than
	// reconstruct them — one issue lifecycle, one trace read, two product faces.
	errorTrackingModule := implerrortracking.NewModule(
		implerrortracking.NewStore(sqlstore),
		implerrortracking.NewNoopSink(),
		implerrortracking.WithRetention(errorTrackingRetention()),
	)
	traceDetailModule := impltracedetail.NewModule(impltracedetail.NewTraceStore(telemetryStore), providerSettings, config.TraceDetail)
	sentryModule := implsentry.NewModule(
		implsentry.NewProjectStore(sqlstore),
		implsentry.NewEventStore(telemetryStore),
		errorTrackingModule,
		implsentry.Config{
			IngestSecret: errorTrackingIngestSecret(),
			Host:         sentryIngestHost(),
			CapturePII:   errorTrackingCapturePII(),
		},
	)

	return Modules{
		OrgGetter:                orgGetter,
		OrgSetter:                orgSetter,
		Preference:               implpreference.NewModule(implpreference.NewStore(sqlstore), preferencetypes.NewAvailablePreference()),
		SavedView:                implsavedview.NewModule(sqlstore),
		Apdex:                    implapdex.NewModule(sqlstore),
		Dashboard:                dashboard,
		UserSetter:               userSetter,
		UserGetter:               userGetter,
		RetentionGetter:          retentionGetter,
		QuickFilter:              quickfilter,
		TraceFunnel:              impltracefunnel.NewModule(impltracefunnel.NewStore(sqlstore)),
		RawDataExport:            implrawdataexport.NewModule(querier),
		AuthDomain:               authDomainModule,
		Session:                  implsession.NewModule(providerSettings, authNs, userSetter, userGetter, authDomainModule, tokenizer, orgGetter, authz),
		SpanPercentile:           implspanpercentile.NewModule(querier, providerSettings),
		Services:                 implservices.NewModule(querier, telemetryStore),
		MetricsExplorer:          implmetricsexplorer.NewModule(telemetryStore, telemetryMetadataStore, cache, ruleStore, dashboard, fl, providerSettings, config.MetricsExplorer),
		MetricReductionRule:      metricReductionRule,
		InfraMonitoring:          implinframonitoring.NewModule(telemetryStore, telemetryMetadataStore, querier, fl, providerSettings, config.InfraMonitoring),
		Promote:                  implpromote.NewModule(telemetryMetadataStore, telemetryStore),
		ServiceAccount:           serviceAccount,
		LogsPipeline:             impllogspipeline.NewModule(sqlstore),
		RuleStateHistory:         implrulestatehistory.NewModule(implrulestatehistory.NewStore(telemetryStore, telemetryMetadataStore, providerSettings.Logger)),
		CloudIntegration:         cloudIntegrationModule,
		TraceDetail:              traceDetailModule,
		SpanMapper:               implspanmapper.NewModule(implspanmapper.NewStore(sqlstore), fl),
		LLMPricingRule:           impllmpricingrule.NewModule(impllmpricingrule.NewStore(sqlstore), fl),
		LLMObs:                   impllmobs.NewModule(querier, impllmobs.NewStore(sqlstore)),
		ErrorTracking:            errorTrackingModule,
		ErrorTrackingRevocations: implerrortracking.NewSQLRevocations(sqlstore),
		Sentry:                   sentryModule,
		Tag:                      tagModule,
	}
}
