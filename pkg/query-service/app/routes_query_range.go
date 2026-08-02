package app

import (
	"github.com/hanzoai/o11y/pkg/http/middleware"
	"github.com/hanzoai/o11y/pkg/http/routing"
)

// DUAL DISPATCH. These registrations stay: the standalone server reaches them
// directly, and the composed binary reaches the SAME handlers through the typed
// query-core ops in the repo root's querycore.go (metricsQueryRange,
// queryRangeFormat), which relay here. Deleting either half drops one of the two
// deployments.

// mountQueryRange registers the metrics range read. 1 route.
func (aH *APIHandler) mountQueryRange(router routing.Router, am *middleware.AuthZ) {
	router.Get("/v1/o11y/query_range", am.ViewAccess(aH.queryRangeMetrics))
}

// mountQueryRangeFormat registers the query-builder format op on the shared
// /v1/o11y subrouter owned by RegisterQueryRangeV3Routes. 1 route.
func (aH *APIHandler) mountQueryRangeFormat(subRouter routing.Router, am *middleware.AuthZ) {
	subRouter.Post("/query_range/format", am.ViewAccess(aH.QueryRangeV3Format))
}
