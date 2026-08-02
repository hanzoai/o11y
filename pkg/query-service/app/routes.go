package app

import (
	"github.com/hanzoai/o11y/pkg/http/middleware"
	"github.com/hanzoai/o11y/pkg/http/routing"
)

// This file is the route index. Every HTTP route the query-service serves is
// declared by exactly one routes_<resource>.go mount function, and every mount
// function is called from here. http_handler.go holds handler bodies only.
//
// Registration order is load-bearing: gorilla/mux is first-match, so a route
// registered earlier wins over a later one that also matches. The call order
// below reproduces the original single-file order exactly.
//
// Grouping by resource is safe because every group owns a distinct *literal*
// first path segment after /v1/o11y — no group registers a path variable in
// that position — so two routes from different groups can never match the same
// request. Order that does decide which handler runs is always intra-group, is
// preserved verbatim inside the resource file, and carries a comment there
// (routes_traces.go, routes_logs.go, routes_trace_funnels.go,
// routes_integrations.go).
//
// 134 routes total.

// RegisterRoutes registers routes for this handler on the given router.
// 44 routes, all mounted on the root router with fully-qualified paths.
func (aH *APIHandler) RegisterRoutes(router routing.Router, am *middleware.AuthZ) {
	aH.mountQueryRange(router, am)
	aH.mountMisc(router, am)
	aH.mountRules(router, am)
	aH.mountExplorer(router, am)
	aH.mountServices(router, am)
	aH.mountTraces(router, am)
	aH.mountSettings(router, am)
	aH.mountErrors(router, am)
	aH.mountOrgs(router, am)
	aH.mountLicenses(router, am)
}

// RegisterQueryRangeV3Routes mounts the query-builder surface. 8 routes.
// All five mounts share ONE /v1/o11y subrouter — the same instance the original
// single-file version built — so the parent router sees one prefix route here,
// exactly as before.
func (aH *APIHandler) RegisterQueryRangeV3Routes(router routing.Router, am *middleware.AuthZ) {
	subRouter := router.Group("/v1/o11y")
	aH.mountAutocomplete(subRouter, am)
	aH.mountQueryRangeFormat(subRouter, am)
	aH.mountFilterSuggestions(subRouter, am)
	aH.mountQueryProgress(subRouter, am)
	aH.mountLogsLivetail(subRouter, am)
}

// RegisterInfraMetricsRoutes mounts the infra-monitoring surface. 34 routes.
// Each resource owns its own /v1/o11y/<resource> subrouter, as before.
func (aH *APIHandler) RegisterInfraMetricsRoutes(router routing.Router, am *middleware.AuthZ) {
	aH.mountHosts(router, am)
	aH.mountProcesses(router, am)
	aH.mountPods(router, am)
	aH.mountPvcs(router, am)
	aH.mountNodes(router, am)
	aH.mountNamespaces(router, am)
	aH.mountClusters(router, am)
	aH.mountDeployments(router, am)
	aH.mountDaemonsets(router, am)
	aH.mountStatefulsets(router, am)
	aH.mountJobs(router, am)
	aH.mountInfraOnboarding(router, am)
}

// RegisterWebSocketPaths mounts the non-/v1/o11y websocket surface. 1 route.
func (aH *APIHandler) RegisterWebSocketPaths(router routing.Router, am *middleware.AuthZ) {
	subRouter := router.Group("/ws")
	aH.mountWebSocketQueryProgress(subRouter, am)
}

// RegisterQueryRangeV4Routes mounts metric metadata. 1 route.
func (aH *APIHandler) RegisterQueryRangeV4Routes(router routing.Router, am *middleware.AuthZ) {
	subRouter := router.Group("/v1/o11y")
	aH.mountMetric(subRouter, am)
}

// RegisterLogsRoutes mounts the logs surface. 7 routes.
// (The 8th logs route, /logs/livetail, is mounted by RegisterQueryRangeV3Routes
// on the shared /v1/o11y subrouter — see routes_logs.go.)
func (aH *APIHandler) RegisterLogsRoutes(router routing.Router, am *middleware.AuthZ) {
	aH.mountLogs(router, am)
}

// RegisterIntegrationRoutes mounts the integrations surface. 5 routes.
func (aH *APIHandler) RegisterIntegrationRoutes(router routing.Router, am *middleware.AuthZ) {
	aH.mountIntegrations(router, am)
}

// RegisterMessagingQueuesRoutes mounts the messaging-queues surface. 14 routes.
func (aH *APIHandler) RegisterMessagingQueuesRoutes(router routing.Router, am *middleware.AuthZ) {
	aH.mountMessagingQueues(router, am)
}

// RegisterThirdPartyApiRoutes mounts the third-party-apis surface. 2 routes.
func (aH *APIHandler) RegisterThirdPartyApiRoutes(router routing.Router, am *middleware.AuthZ) {
	aH.mountThirdPartyApis(router, am)
}

// RegisterTraceFunnelsRoutes mounts the trace-funnels surface. 18 routes.
func (aH *APIHandler) RegisterTraceFunnelsRoutes(router routing.Router, am *middleware.AuthZ) {
	aH.mountTraceFunnels(router, am)
}
