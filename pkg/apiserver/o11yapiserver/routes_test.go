package o11yapiserver

import (
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/hanzoai/o11y/pkg/alertmanager"
	"github.com/hanzoai/o11y/pkg/authz"
	"github.com/hanzoai/o11y/pkg/factory"
	"github.com/hanzoai/o11y/pkg/flagger"
	"github.com/hanzoai/o11y/pkg/gateway"
	"github.com/hanzoai/o11y/pkg/global"
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
	"github.com/hanzoai/o11y/pkg/zeus"
	"github.com/zap-proto/zip"
)

// THE API SERVER'S CENSUS.
//
// This package registers the larger half of the service's HTTP surface, and it
// registers it by calling 32 functions from one place. An uncalled func is legal
// Go, so a slice that stops being called compiles, passes every test it has, and
// serves nothing — which is exactly what this repo already shipped once, with 83
// typed ops present in the source and reaching no router at all. Only arithmetic
// catches that, so the arithmetic lives here.
//
// If this number moves, a route was added or removed and someone has to say
// which. It is not a target to satisfy by editing the constant.
const wantAPIServerRoutes = 233

// every handler field satisfied by an embedded nil interface: the route table is
// a fact about REGISTRATION, and registration only takes method values off these
// — it never calls one. Standing up 33 real modules to count routes would test
// the modules, not the routes.
func censusProvider() *provider {
	return &provider{
		authzMiddleware:            middleware.NewAuthZ(slog.Default(), nil, nil),
		authzService:               struct{ authz.AuthZ }{},
		orgHandler:                 struct{ organization.Handler }{},
		userHandler:                struct{ user.Handler }{},
		sessionHandler:             struct{ session.Handler }{},
		authDomainHandler:          struct{ authdomain.Handler }{},
		preferenceHandler:          struct{ preference.Handler }{},
		globalHandler:              struct{ global.Handler }{},
		promoteHandler:             struct{ promote.Handler }{},
		flaggerHandler:             struct{ flagger.Handler }{},
		dashboardModule:            struct{ dashboard.Module }{},
		dashboardHandler:           struct{ dashboard.Handler }{},
		metricsExplorerHandler:     struct{ metricsexplorer.Handler }{},
		metricReductionRuleHandler: struct{ metricreductionrule.Handler }{},
		infraMonitoringHandler:     struct{ inframonitoring.Handler }{},
		gatewayHandler:             struct{ gateway.Handler }{},
		fieldsHandler:              struct{ fields.Handler }{},
		authzHandler:               struct{ authz.Handler }{},
		rawDataExportHandler:       struct{ rawdataexport.Handler }{},
		zeusHandler:                struct{ zeus.Handler }{},
		querierHandler:             struct{ querier.Handler }{},
		serviceAccountHandler:      struct{ serviceaccount.Handler }{},
		factoryHandler:             struct{ factory.Handler }{},
		cloudIntegrationHandler:    struct{ cloudintegration.Handler }{},
		ruleStateHistoryHandler:    struct{ rulestatehistory.Handler }{},
		spanMapperHandler:          struct{ spanmapper.Handler }{},
		alertmanagerHandler:        struct{ alertmanager.Handler }{},
		traceDetailHandler:         struct{ tracedetail.Handler }{},
		rulerHandler:               struct{ ruler.Handler }{},
		llmPricingRuleHandler:      struct{ llmpricingrule.Handler }{},
		llmObsHandler:              struct{ llmobs.Handler }{},
		errorTrackingHandler:       struct{ errortracking.Handler }{},
		sentryHandler:              struct{ sentry.Handler }{},
		statsHandler:               struct{ statsreporter.Handler }{},
	}
}

func TestEveryAPIServerRouteIsRegistered(t *testing.T) {
	app := zip.New(zip.Config{DisableStartupMessage: true})
	router := routing.New(app.Group(""), nil)
	censusProvider().AddToRouter(router)

	table := router.Table().Routes()
	if len(table) != wantAPIServerRoutes {
		t.Fatalf("AddToRouter registered %d routes, want %d", len(table), wantAPIServerRoutes)
	}

	// The table is what registration recorded; the router is what will answer.
	// They must be the same set, or the census is measuring the wrong thing.
	onRouter := 0
	for _, route := range app.Fiber().GetRoutes(true) {
		if route.Method == http.MethodHead || route.Method == http.MethodOptions {
			continue
		}
		onRouter++
	}
	if onRouter != len(table) {
		t.Fatalf("router holds %d routes, the table recorded %d", onRouter, len(table))
	}

	// Every route answers on one of the three public roots — the two faces and
	// the ingest endpoint — and carries no leftover spelling from the tree this
	// replaced.
	for _, route := range table {
		switch {
		case strings.HasPrefix(route.Path, "/v1/o11y"):
		case strings.HasPrefix(route.Path, "/v1/o11y/sentinel"):
		case strings.HasPrefix(route.Path, "/v1/event"):
		default:
			t.Errorf("%s %s is on none of the public roots", route.Method, route.Path)
		}
	}
}
