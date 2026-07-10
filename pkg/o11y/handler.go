package o11y

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/hanzoai/o11y/pkg/alertmanager"
	"github.com/hanzoai/o11y/pkg/alertmanager/o11yalertmanager"
	"github.com/hanzoai/o11y/pkg/analytics"
	"github.com/hanzoai/o11y/pkg/authz"
	"github.com/hanzoai/o11y/pkg/authz/o11yauthzapi"
	"github.com/hanzoai/o11y/pkg/factory"
	"github.com/hanzoai/o11y/pkg/flagger"
	"github.com/hanzoai/o11y/pkg/gateway"
	"github.com/hanzoai/o11y/pkg/global"
	"github.com/hanzoai/o11y/pkg/global/o11yglobal"
	"github.com/hanzoai/o11y/pkg/licensing"
	"github.com/hanzoai/o11y/pkg/modules/apdex"
	"github.com/hanzoai/o11y/pkg/modules/apdex/implapdex"
	"github.com/hanzoai/o11y/pkg/modules/cloudintegration"
	"github.com/hanzoai/o11y/pkg/modules/cloudintegration/implcloudintegration"
	"github.com/hanzoai/o11y/pkg/modules/dashboard"
	"github.com/hanzoai/o11y/pkg/modules/dashboard/impldashboard"
	"github.com/hanzoai/o11y/pkg/modules/errortracking"
	"github.com/hanzoai/o11y/pkg/modules/errortracking/implerrortracking"
	"github.com/hanzoai/o11y/pkg/modules/fields"
	"github.com/hanzoai/o11y/pkg/modules/fields/implfields"
	"github.com/hanzoai/o11y/pkg/modules/inframonitoring"
	"github.com/hanzoai/o11y/pkg/modules/inframonitoring/implinframonitoring"
	"github.com/hanzoai/o11y/pkg/modules/llmobs"
	"github.com/hanzoai/o11y/pkg/modules/llmobs/impllmobs"
	"github.com/hanzoai/o11y/pkg/modules/llmpricingrule"
	"github.com/hanzoai/o11y/pkg/modules/llmpricingrule/impllmpricingrule"
	"github.com/hanzoai/o11y/pkg/modules/metricreductionrule"
	"github.com/hanzoai/o11y/pkg/modules/metricreductionrule/implmetricreductionrule"
	"github.com/hanzoai/o11y/pkg/modules/metricsexplorer"
	"github.com/hanzoai/o11y/pkg/modules/metricsexplorer/implmetricsexplorer"
	"github.com/hanzoai/o11y/pkg/modules/quickfilter"
	"github.com/hanzoai/o11y/pkg/modules/quickfilter/implquickfilter"
	"github.com/hanzoai/o11y/pkg/modules/rawdataexport"
	"github.com/hanzoai/o11y/pkg/modules/rawdataexport/implrawdataexport"
	"github.com/hanzoai/o11y/pkg/modules/rulestatehistory"
	"github.com/hanzoai/o11y/pkg/modules/rulestatehistory/implrulestatehistory"
	"github.com/hanzoai/o11y/pkg/modules/savedview"
	"github.com/hanzoai/o11y/pkg/modules/savedview/implsavedview"
	"github.com/hanzoai/o11y/pkg/modules/serviceaccount"
	"github.com/hanzoai/o11y/pkg/modules/serviceaccount/implserviceaccount"
	"github.com/hanzoai/o11y/pkg/modules/services"
	"github.com/hanzoai/o11y/pkg/modules/services/implservices"
	"github.com/hanzoai/o11y/pkg/modules/spanmapper"
	"github.com/hanzoai/o11y/pkg/modules/spanmapper/implspanmapper"
	"github.com/hanzoai/o11y/pkg/modules/spanpercentile"
	"github.com/hanzoai/o11y/pkg/modules/spanpercentile/implspanpercentile"
	"github.com/hanzoai/o11y/pkg/modules/tracedetail"
	"github.com/hanzoai/o11y/pkg/modules/tracedetail/impltracedetail"
	"github.com/hanzoai/o11y/pkg/modules/tracefunnel"
	"github.com/hanzoai/o11y/pkg/modules/tracefunnel/impltracefunnel"
	"github.com/hanzoai/o11y/pkg/querier"
	"github.com/hanzoai/o11y/pkg/ruler"
	"github.com/hanzoai/o11y/pkg/ruler/o11yruler"
	"github.com/hanzoai/o11y/pkg/statsreporter"
	"github.com/hanzoai/o11y/pkg/types/telemetrytypes"
	"github.com/hanzoai/o11y/pkg/zeus"
)

type Handlers struct {
	SavedView               savedview.Handler
	Apdex                   apdex.Handler
	Dashboard               dashboard.Handler
	QuickFilter             quickfilter.Handler
	TraceFunnel             tracefunnel.Handler
	RawDataExport           rawdataexport.Handler
	SpanPercentile          spanpercentile.Handler
	Services                services.Handler
	MetricsExplorer         metricsexplorer.Handler
	MetricReductionRule     metricreductionrule.Handler
	InfraMonitoring         inframonitoring.Handler
	Global                  global.Handler
	FlaggerHandler          flagger.Handler
	GatewayHandler          gateway.Handler
	Fields                  fields.Handler
	AuthzHandler            authz.Handler
	ZeusHandler             zeus.Handler
	QuerierHandler          querier.Handler
	ServiceAccountHandler   serviceaccount.Handler
	RegistryHandler         factory.Handler
	CloudIntegrationHandler cloudintegration.Handler
	RuleStateHistory        rulestatehistory.Handler
	SpanMapperHandler       spanmapper.Handler
	AlertmanagerHandler     alertmanager.Handler
	TraceDetail             tracedetail.Handler
	RulerHandler            ruler.Handler
	LLMPricingRuleHandler   llmpricingrule.Handler
	LLMObsHandler           llmobs.Handler
	ErrorTrackingHandler    errortracking.Handler
	StatsHandler            statsreporter.Handler
}

func NewHandlers(
	modules Modules,
	providerSettings factory.ProviderSettings,
	analytics analytics.Analytics,
	querierHandler querier.Handler,
	licensing licensing.Licensing,
	global global.Global,
	flaggerService flagger.Flagger,
	gatewayService gateway.Gateway,
	telemetryMetadataStore telemetrytypes.MetadataStore,
	authz authz.AuthZ,
	zeusService zeus.Zeus,
	registryHandler factory.Handler,
	alertmanagerService alertmanager.Alertmanager,
	rulerService ruler.Ruler,
	statsAggregator statsreporter.Aggregator,
) Handlers {
	return Handlers{
		SavedView:               implsavedview.NewHandler(modules.SavedView),
		Apdex:                   implapdex.NewHandler(modules.Apdex),
		Dashboard:               impldashboard.NewHandler(modules.Dashboard, providerSettings, authz),
		QuickFilter:             implquickfilter.NewHandler(modules.QuickFilter),
		TraceFunnel:             impltracefunnel.NewHandler(modules.TraceFunnel),
		RawDataExport:           implrawdataexport.NewHandler(modules.RawDataExport),
		Services:                implservices.NewHandler(modules.Services),
		MetricsExplorer:         implmetricsexplorer.NewHandler(modules.MetricsExplorer),
		MetricReductionRule:     implmetricreductionrule.NewHandler(modules.MetricReductionRule),
		InfraMonitoring:         implinframonitoring.NewHandler(modules.InfraMonitoring),
		SpanPercentile:          implspanpercentile.NewHandler(modules.SpanPercentile),
		Global:                  o11yglobal.NewHandler(global),
		FlaggerHandler:          flagger.NewHandler(flaggerService),
		GatewayHandler:          gateway.NewHandler(gatewayService),
		Fields:                  implfields.NewHandler(providerSettings, telemetryMetadataStore),
		AuthzHandler:            o11yauthzapi.NewHandler(authz),
		ZeusHandler:             zeus.NewHandler(zeusService, licensing),
		QuerierHandler:          querierHandler,
		ServiceAccountHandler:   implserviceaccount.NewHandler(modules.ServiceAccount),
		RegistryHandler:         registryHandler,
		RuleStateHistory:        implrulestatehistory.NewHandler(modules.RuleStateHistory),
		CloudIntegrationHandler: implcloudintegration.NewHandler(modules.CloudIntegration),
		SpanMapperHandler:       implspanmapper.NewHandler(modules.SpanMapper),
		AlertmanagerHandler:     o11yalertmanager.NewHandler(alertmanagerService),
		TraceDetail:             impltracedetail.NewHandler(modules.TraceDetail),
		RulerHandler:            o11yruler.NewHandler(rulerService),
		LLMPricingRuleHandler:   impllmpricingrule.NewHandler(modules.LLMPricingRule),
		LLMObsHandler:           impllmobs.NewHandler(modules.LLMObs),
		ErrorTrackingHandler:    implerrortracking.NewHandler(modules.ErrorTracking, errorTrackingIngestSecret(), errorTrackingCapturePII(), modules.ErrorTrackingRevocations),
		StatsHandler:            statsreporter.NewHandler(statsAggregator),
	}
}

// errorTrackingIngestSecret is the platform secret used to verify Sentry DSN keys
// on the public error-ingest endpoints. It is sourced from KMS (synced to this env
// var via a KMSSecret CRD) — never committed, never plaintext at rest. When unset,
// the ingest endpoints fail closed (503) while the IAM-scoped read endpoints keep
// working.
func errorTrackingIngestSecret() []byte {
	return []byte(os.Getenv("O11Y_ERRORTRACKING_INGEST_SECRET"))
}

// errorTrackingCapturePII reports whether the error-ingest path retains end-user
// PII (email/IP). Default false = scrub (fail-secure), mirroring the llmobs
// O11Y_GENAI_CAPTURE_MESSAGES precedent. Secrets are always redacted regardless.
func errorTrackingCapturePII() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("O11Y_ERRORTRACKING_CAPTURE_PII")))
	return v == "true" || v == "1" || v == "yes"
}

// errorTrackingRetention is the age past which resolved-or-stale issues are swept.
// Default 90 days; O11Y_ERRORTRACKING_RETENTION_DAYS overrides; 0 disables the sweep.
func errorTrackingRetention() time.Duration {
	days := 90
	if v := strings.TrimSpace(os.Getenv("O11Y_ERRORTRACKING_RETENTION_DAYS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			days = n
		}
	}
	return time.Duration(days) * 24 * time.Hour
}
