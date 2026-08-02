package app

import (
	"github.com/hanzoai/o11y/pkg/http/middleware"
	"github.com/hanzoai/o11y/pkg/http/routing"
)

// Single-route resources that do not justify a file of their own. 14 routes.
// Each mount below is called from the entry point that owns its router, so the
// registration order of the original single-file version is preserved.
//
// If any of these grows a second route, give it its own routes_<resource>.go.
//
// TWO DISPATCHES, ONE IMPLEMENTATION. These registrations stay: the standalone
// server has no native router to register an op on. The composed binary reaches
// the SAME handlers through the typed ops in the repo root's platform.go, which
// relay here — so the gates named below (OpenAccess, ViewAccess) stay the one
// place access is decided, and deleting either half drops one of the two
// deployments.

// mountMisc registers the root-router singletons. 11 routes:
// query, variables, event, usage, dependency_graph, version, health, disks,
// register, span_percentile, query_filter.
func (aH *APIHandler) mountMisc(router routing.Router, am *middleware.AuthZ) {
	router.Get("/v1/o11y/query", am.ViewAccess(aH.queryMetrics))

	// dashboards + dashboards/{id} + dashboards/{id}/lock are served by
	// o11yapiserver/dashboard.go (formerly /api/v2/dashboards). Highest version wins.
	router.Post("/v1/o11y/variables/query", am.ViewAccess(aH.queryDashboardVarsV2))

	router.Post("/v1/o11y/event", am.ViewAccess(aH.registerEvent))

	router.Get("/v1/o11y/usage", am.ViewAccess(aH.getUsage))
	router.Post("/v1/o11y/dependency_graph", am.ViewAccess(aH.dependencyGraph))

	router.Get("/v1/o11y/version", am.OpenAccess(aH.getVersion))
	// features is served by o11yapiserver/flagger.go (formerly /api/v2/features).
	router.Get("/v1/o11y/health", am.OpenAccess(aH.getHealth))

	router.Get("/v1/o11y/disks", am.ViewAccess(aH.getDisks))

	router.Post("/v1/o11y/span_percentile", am.ViewAccess(aH.O11y.Handlers.SpanPercentile.GetSpanPercentileDetails))

	// Query Filter Analyzer api used to extract metric names and grouping columns from a query
	router.Post("/v1/o11y/query_filter/analyze", am.ViewAccess(aH.QueryParserAPI.AnalyzeQueryFilter))
}

// mountFilterSuggestions registers query-builder filter suggestions on the
// shared /v1/o11y subrouter owned by RegisterQueryRangeV3Routes. 1 route.
func (aH *APIHandler) mountFilterSuggestions(subRouter routing.Router, am *middleware.AuthZ) {
	subRouter.Get("/filter_suggestions", am.ViewAccess(aH.getQueryBuilderSuggestions))
}

// mountInfraOnboarding registers the k8s onboarding status probe. 1 route.
// Owns its own /v1/o11y/infra_onboarding subrouter, as before.
func (aH *APIHandler) mountInfraOnboarding(router routing.Router, am *middleware.AuthZ) {
	infraOnboardingSubRouter := router.Group("/v1/o11y/infra_onboarding")
	infraOnboardingSubRouter.Get("/k8s/status", am.ViewAccess(aH.getK8sInfraOnboardingStatus))
}

// mountMetric registers metric metadata on the /v1/o11y subrouter owned by
// RegisterQueryRangeV4Routes. 1 route.
func (aH *APIHandler) mountMetric(subRouter routing.Router, am *middleware.AuthZ) {
	subRouter.Get("/metric/metric_metadata", am.ViewAccess(aH.getMetricMetadata))
}
