package app

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/hanzoai/o11y/pkg/http/middleware"
)

// mountQueryRange registers the metrics range read. 1 route.
func (aH *APIHandler) mountQueryRange(router *mux.Router, am *middleware.AuthZ) {
	router.HandleFunc("/v1/o11y/query_range", am.ViewAccess(aH.queryRangeMetrics)).Methods(http.MethodGet)
}

// mountQueryRangeFormat registers the query-builder format op on the shared
// /v1/o11y subrouter owned by RegisterQueryRangeV3Routes. 1 route.
func (aH *APIHandler) mountQueryRangeFormat(subRouter *mux.Router, am *middleware.AuthZ) {
	subRouter.HandleFunc("/query_range/format", am.ViewAccess(aH.QueryRangeV3Format)).Methods(http.MethodPost)
}
