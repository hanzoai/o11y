package app

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/hanzoai/o11y/pkg/http/middleware"
)

// DUAL DISPATCH. These registrations stay: the standalone server reaches them
// directly, and the composed binary reaches the SAME handlers through the typed
// query-core ops in the repo root's querycore.go (metricsQueryRange,
// queryRangeFormat), which relay here. Deleting either half drops one of the two
// deployments.

// mountQueryRange registers the metrics range read. 1 route.
func (aH *APIHandler) mountQueryRange(router *mux.Router, am *middleware.AuthZ) {
	router.HandleFunc("/v1/o11y/query_range", am.ViewAccess(aH.queryRangeMetrics)).Methods(http.MethodGet)
}

// mountQueryRangeFormat registers the query-builder format op on the shared
// /v1/o11y subrouter owned by RegisterQueryRangeV3Routes. 1 route.
func (aH *APIHandler) mountQueryRangeFormat(subRouter *mux.Router, am *middleware.AuthZ) {
	subRouter.HandleFunc("/query_range/format", am.ViewAccess(aH.QueryRangeV3Format)).Methods(http.MethodPost)
}
