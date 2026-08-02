package app

import (
	"github.com/hanzoai/o11y/pkg/http/middleware"
	"github.com/hanzoai/o11y/pkg/http/routing"
)

// mountTraces registers trace search and the trace field catalog. 3 routes.
//
// ORDER IS LOAD-BEARING. /traces/{traceId} is registered FIRST, exactly as in
// the original file. gorilla/mux is first-match, so GET /v1/o11y/traces/fields
// binds traceId="fields" and is served by SearchTraces — the GET /traces/fields
// registration below is only reachable if the first one is removed. That is the
// behavior today; preserved verbatim. Do not reorder these two without deciding
// which handler is supposed to win.
func (aH *APIHandler) mountTraces(router routing.Router, am *middleware.AuthZ) {
	router.Get("/v1/o11y/traces/{traceId}", am.ViewAccess(aH.SearchTraces))

	router.Get("/v1/o11y/traces/fields", am.ViewAccess(aH.traceFields))
	router.Post("/v1/o11y/traces/fields", am.EditAccess(aH.updateTraceField))
}
