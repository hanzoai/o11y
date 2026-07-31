package app

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/hanzoai/o11y/pkg/http/middleware"
)

// Single-route resources that do not justify a file of their own. 14 routes.
// Each mount below is called from the entry point that owns its router, so the
// registration order of the original single-file version is preserved.
//
// If any of these grows a second route, give it its own routes_<resource>.go.

// mountMisc registers the root-router singletons. 11 routes:
// query, variables, event, usage, dependency_graph, version, health, disks,
// register, span_percentile, query_filter.
func (aH *APIHandler) mountMisc(router *mux.Router, am *middleware.AuthZ) {
	router.HandleFunc("/v1/o11y/query", am.ViewAccess(aH.queryMetrics)).Methods(http.MethodGet)

	// dashboards + dashboards/{id} + dashboards/{id}/lock are served by
	// o11yapiserver/dashboard.go (formerly /api/v2/dashboards). Highest version wins.
	router.HandleFunc("/v1/o11y/variables/query", am.ViewAccess(aH.queryDashboardVarsV2)).Methods(http.MethodPost)

	router.HandleFunc("/v1/o11y/event", am.ViewAccess(aH.registerEvent)).Methods(http.MethodPost)

	router.HandleFunc("/v1/o11y/usage", am.ViewAccess(aH.getUsage)).Methods(http.MethodGet)
	router.HandleFunc("/v1/o11y/dependency_graph", am.ViewAccess(aH.dependencyGraph)).Methods(http.MethodPost)

	router.HandleFunc("/v1/o11y/version", am.OpenAccess(aH.getVersion)).Methods(http.MethodGet)
	// features is served by o11yapiserver/flagger.go (formerly /api/v2/features).
	router.HandleFunc("/v1/o11y/health", am.OpenAccess(aH.getHealth)).Methods(http.MethodGet)

	router.HandleFunc("/v1/o11y/disks", am.ViewAccess(aH.getDisks)).Methods(http.MethodGet)

	router.HandleFunc("/v1/o11y/register", am.OpenAccess(aH.registerUser)).Methods(http.MethodPost)

	router.HandleFunc("/v1/o11y/span_percentile", am.ViewAccess(aH.O11y.Handlers.SpanPercentile.GetSpanPercentileDetails)).Methods(http.MethodPost)

	// Query Filter Analyzer api used to extract metric names and grouping columns from a query
	router.HandleFunc("/v1/o11y/query_filter/analyze", am.ViewAccess(aH.QueryParserAPI.AnalyzeQueryFilter)).Methods(http.MethodPost)
}

// mountFilterSuggestions registers query-builder filter suggestions on the
// shared /v1/o11y subrouter owned by RegisterQueryRangeV3Routes. 1 route.
func (aH *APIHandler) mountFilterSuggestions(subRouter *mux.Router, am *middleware.AuthZ) {
	subRouter.HandleFunc("/filter_suggestions", am.ViewAccess(aH.getQueryBuilderSuggestions)).Methods(http.MethodGet)
}

// mountInfraOnboarding registers the k8s onboarding status probe. 1 route.
// Owns its own /v1/o11y/infra_onboarding subrouter, as before.
func (aH *APIHandler) mountInfraOnboarding(router *mux.Router, am *middleware.AuthZ) {
	infraOnboardingSubRouter := router.PathPrefix("/v1/o11y/infra_onboarding").Subrouter()
	infraOnboardingSubRouter.HandleFunc("/k8s/status", am.ViewAccess(aH.getK8sInfraOnboardingStatus)).Methods(http.MethodGet)
}

// mountMetric registers metric metadata on the /v1/o11y subrouter owned by
// RegisterQueryRangeV4Routes. 1 route.
func (aH *APIHandler) mountMetric(subRouter *mux.Router, am *middleware.AuthZ) {
	subRouter.HandleFunc("/metric/metric_metadata", am.ViewAccess(aH.getMetricMetadata)).Methods(http.MethodGet)
}
