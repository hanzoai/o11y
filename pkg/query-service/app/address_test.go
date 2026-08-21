package app

import (
	"log/slog"
	"net/http"
	"testing"

	"github.com/hanzoai/o11y/pkg/alertmanager"
	"github.com/hanzoai/o11y/pkg/authz"
	"github.com/hanzoai/o11y/pkg/factory"
	"github.com/hanzoai/o11y/pkg/flagger"
	"github.com/hanzoai/o11y/pkg/gateway"
	"github.com/hanzoai/o11y/pkg/global"
	"github.com/hanzoai/o11y/pkg/http/middleware"
	"github.com/hanzoai/o11y/pkg/http/routing"
	"github.com/hanzoai/o11y/pkg/modules/apdex"
	"github.com/hanzoai/o11y/pkg/modules/cloudintegration"
	"github.com/hanzoai/o11y/pkg/modules/dashboard"
	"github.com/hanzoai/o11y/pkg/modules/errortracking"
	"github.com/hanzoai/o11y/pkg/modules/fields"
	"github.com/hanzoai/o11y/pkg/modules/inframonitoring"
	"github.com/hanzoai/o11y/pkg/modules/llmobs"
	"github.com/hanzoai/o11y/pkg/modules/llmpricingrule"
	"github.com/hanzoai/o11y/pkg/modules/metricreductionrule"
	"github.com/hanzoai/o11y/pkg/modules/metricsexplorer"
	"github.com/hanzoai/o11y/pkg/modules/quickfilter"
	"github.com/hanzoai/o11y/pkg/modules/rawdataexport"
	"github.com/hanzoai/o11y/pkg/modules/rulestatehistory"
	"github.com/hanzoai/o11y/pkg/modules/savedview"
	"github.com/hanzoai/o11y/pkg/modules/sentry"
	"github.com/hanzoai/o11y/pkg/modules/serviceaccount"
	"github.com/hanzoai/o11y/pkg/modules/services"
	"github.com/hanzoai/o11y/pkg/modules/spanmapper"
	"github.com/hanzoai/o11y/pkg/modules/spanpercentile"
	"github.com/hanzoai/o11y/pkg/modules/tracedetail"
	"github.com/hanzoai/o11y/pkg/modules/tracefunnel"
	o11yrt "github.com/hanzoai/o11y/pkg/o11y"
	"github.com/hanzoai/o11y/pkg/querier"
	"github.com/hanzoai/o11y/pkg/ruler"
	"github.com/hanzoai/o11y/pkg/statsreporter"
	"github.com/hanzoai/o11y/pkg/zeus"
	"github.com/zap-proto/zip"

	published "github.com/hanzoai/o11y"
)

// wantQueryServiceRoutes is this registrar's half of the surface, and it is the
// term the equality argument was missing.
//
// The sibling at pkg/apiserver/o11yapiserver/address_test.go argues set equality
// from four facts: registered ⊆ declared on both halves, 233 + 133 = 366, and a
// duplicate registration panicking at boot. Three of those four were measured;
// 134 was a sentence. So a route deleted HERE left both address tests green — the
// remaining ones are all still declared — while the declaration went on naming an
// address nothing serves. That gap costs more since the seam started resolving by
// name: it used to be the runtime's 404, and it is now a 503 that says the
// runtime does not serve a route the document publishes.
//
// Counting is the right instrument for exactly this one fact and no other: what
// the sets CONTAIN is checked by name below, and a cardinality is what closes
// "⊆ on both halves" into "=". It is stated the way its sibling states its own so
// the arithmetic reads as one argument in two files.
const wantQueryServiceRoutes = 133

// BY NAME, NOT BY COUNT — see the argument in full at
// pkg/apiserver/o11yapiserver/address_test.go. This is the other half of it: the
// addresses this package registers must each be one the module declares, and
// must each resolve through the table by that same name, which is how the
// declaration reaches them now that it names an address instead of speaking HTTP
// to the whole router.
func TestEveryRegisteredAddressIsDeclaredAndResolves(t *testing.T) {
	z := zip.New(zip.Config{DisableStartupMessage: true})
	r := routing.New(z.Group(""), nil)
	aH := &APIHandler{O11y: &o11yrt.O11y{Handlers: o11yrt.Handlers{
		SavedView:               struct{ savedview.Handler }{},
		Apdex:                   struct{ apdex.Handler }{},
		Dashboard:               struct{ dashboard.Handler }{},
		QuickFilter:             struct{ quickfilter.Handler }{},
		TraceFunnel:             struct{ tracefunnel.Handler }{},
		RawDataExport:           struct{ rawdataexport.Handler }{},
		SpanPercentile:          struct{ spanpercentile.Handler }{},
		Services:                struct{ services.Handler }{},
		MetricsExplorer:         struct{ metricsexplorer.Handler }{},
		MetricReductionRule:     struct{ metricreductionrule.Handler }{},
		InfraMonitoring:         struct{ inframonitoring.Handler }{},
		Global:                  struct{ global.Handler }{},
		FlaggerHandler:          struct{ flagger.Handler }{},
		GatewayHandler:          struct{ gateway.Handler }{},
		Fields:                  struct{ fields.Handler }{},
		AuthzHandler:            struct{ authz.Handler }{},
		ZeusHandler:             struct{ zeus.Handler }{},
		QuerierHandler:          struct{ querier.Handler }{},
		ServiceAccountHandler:   struct{ serviceaccount.Handler }{},
		RegistryHandler:         struct{ factory.Handler }{},
		CloudIntegrationHandler: struct{ cloudintegration.Handler }{},
		RuleStateHistory:        struct{ rulestatehistory.Handler }{},
		SpanMapperHandler:       struct{ spanmapper.Handler }{},
		AlertmanagerHandler:     struct{ alertmanager.Handler }{},
		TraceDetail:             struct{ tracedetail.Handler }{},
		RulerHandler:            struct{ ruler.Handler }{},
		LLMPricingRuleHandler:   struct{ llmpricingrule.Handler }{},
		LLMObsHandler:           struct{ llmobs.Handler }{},
		ErrorTrackingHandler:    struct{ errortracking.Handler }{},
		SentryHandler:           struct{ sentry.Handler }{},
		StatsHandler:            struct{ statsreporter.Handler }{},
	}}}
	am := middleware.NewAuthZ(slog.Default(), nil, nil)
	aH.RegisterRoutes(r, am)
	aH.RegisterLogsRoutes(r, am)
	aH.RegisterIntegrationRoutes(r, am)
	aH.RegisterQueryRangeV3Routes(r, am)
	aH.RegisterInfraMetricsRoutes(r, am)
	aH.RegisterQueryRangeV4Routes(r, am)
	aH.RegisterMessagingQueuesRoutes(r, am)
	aH.RegisterThirdPartyApiRoutes(r, am)
	aH.RegisterTraceFunnelsRoutes(r, am)

	d := zip.New(zip.Config{DisableStartupMessage: true})
	if err := published.Mount(d); err != nil {
		t.Fatalf("Mount: %v", err)
	}
	declared := map[string]bool{}
	for _, route := range d.Fiber().GetRoutes(true) {
		if route.Method == http.MethodHead || route.Method == http.MethodOptions {
			continue
		}
		declared[route.Method+" "+zip.Template(route.Path)] = true
	}

	table := r.Table()
	if got := len(table.Routes()); got != wantQueryServiceRoutes {
		t.Errorf("this package registers %d addresses, want %d — the declaration names 366 and "+
			"the other registrar %d, so a move here leaves an address declared and unserved",
			got, wantQueryServiceRoutes, 366-wantQueryServiceRoutes)
	}
	for _, route := range table.Routes() {
		if !declared[route.Method+" "+route.Path] {
			t.Errorf("%s %s is served here and declared nowhere — a caller reaching it "+
				"is in no document, and the seam cannot name it", route.Method, route.Path)
		}
		if table.Handler(route.Method, route.Path) == nil {
			t.Errorf("%s %s registered but does not resolve by name", route.Method, route.Path)
		}
	}
}
