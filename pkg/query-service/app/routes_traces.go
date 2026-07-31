package app

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/hanzoai/o11y/pkg/http/middleware"
)

// mountTraces registers trace search and the trace field catalog. 3 routes.
//
// ORDER IS LOAD-BEARING. /traces/{traceId} is registered FIRST, exactly as in
// the original file. gorilla/mux is first-match, so GET /v1/o11y/traces/fields
// binds traceId="fields" and is served by SearchTraces — the GET /traces/fields
// registration below is only reachable if the first one is removed. That is the
// behavior today; preserved verbatim. Do not reorder these two without deciding
// which handler is supposed to win.
func (aH *APIHandler) mountTraces(router *mux.Router, am *middleware.AuthZ) {
	router.HandleFunc("/v1/o11y/traces/{traceId}", am.ViewAccess(aH.SearchTraces)).Methods(http.MethodGet)

	router.HandleFunc("/v1/o11y/traces/fields", am.ViewAccess(aH.traceFields)).Methods(http.MethodGet)
	router.HandleFunc("/v1/o11y/traces/fields", am.EditAccess(aH.updateTraceField)).Methods(http.MethodPost)
}
