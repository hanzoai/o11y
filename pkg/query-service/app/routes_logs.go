package app

import (
	"github.com/hanzoai/o11y/pkg/http/middleware"
	"github.com/hanzoai/o11y/pkg/http/routing"
)

// mountLogs registers the logs surface: reads, the field catalog, and log
// pipelines. 7 routes on its own /v1/o11y/logs subrouter.
//
// TWO DISPATCHES, ONE IMPLEMENTATION. These registrations stay: the standalone
// server has no native router to register an op on. The composed binary
// reaches the SAME handlers through the typed ops in the repo root's logs.go,
// which relay here — deleting either half drops one of the two deployments.
//
// ORDER IS LOAD-BEARING inside the pipelines block: /pipelines/preview is
// registered before /pipelines/{version}, so POST .../pipelines/preview reaches
// the preview handler rather than binding version="preview". Preserved verbatim.
func (aH *APIHandler) mountLogs(router routing.Router, am *middleware.AuthZ) {
	subRouter := router.Group("/v1/o11y/logs")
	subRouter.Get("", am.ViewAccess(aH.getLogs))
	subRouter.Get("/fields", am.ViewAccess(aH.logFields))
	subRouter.Post("/fields", am.EditAccess(aH.logFieldUpdate))
	subRouter.Get("/aggregate", am.ViewAccess(aH.logAggregate))

	// log pipelines
	subRouter.Post("/pipelines/preview", am.ViewAccess(aH.PreviewLogsPipelinesHandler))
	subRouter.Get("/pipelines/{version}", am.ViewAccess(aH.ListLogsPipelinesHandler))
	subRouter.Post("/pipelines", am.EditAccess(aH.CreateLogsPipeline))
}

// mountLogsLivetail registers live logs on the shared /v1/o11y subrouter owned
// by RegisterQueryRangeV3Routes. 1 route.
//
// It is NOT on the /v1/o11y/logs subrouter above: that subrouter is registered
// first and has no /livetail child, so gorilla/mux falls through to the
// /v1/o11y prefix route and this registration wins. Moving it under mountLogs
// would change nothing for callers but would change which prefix route matches
// — left exactly where it was.
func (aH *APIHandler) mountLogsLivetail(subRouter routing.Router, am *middleware.AuthZ) {
	subRouter.Get("/logs/livetail", am.ViewAccess(aH.O11y.Handlers.QuerierHandler.QueryRawStream))
}
