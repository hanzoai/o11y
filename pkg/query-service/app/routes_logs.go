package app

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/hanzoai/o11y/pkg/http/middleware"
)

// mountLogs registers the logs surface: reads, the field catalog, and log
// pipelines. 7 routes on its own /v1/o11y/logs subrouter.
//
// ORDER IS LOAD-BEARING inside the pipelines block: /pipelines/preview is
// registered before /pipelines/{version}, so POST .../pipelines/preview reaches
// the preview handler rather than binding version="preview". Preserved verbatim.
func (aH *APIHandler) mountLogs(router *mux.Router, am *middleware.AuthZ) {
	subRouter := router.PathPrefix("/v1/o11y/logs").Subrouter()
	subRouter.HandleFunc("", am.ViewAccess(aH.getLogs)).Methods(http.MethodGet)
	subRouter.HandleFunc("/fields", am.ViewAccess(aH.logFields)).Methods(http.MethodGet)
	subRouter.HandleFunc("/fields", am.EditAccess(aH.logFieldUpdate)).Methods(http.MethodPost)
	subRouter.HandleFunc("/aggregate", am.ViewAccess(aH.logAggregate)).Methods(http.MethodGet)

	// log pipelines
	subRouter.HandleFunc("/pipelines/preview", am.ViewAccess(aH.PreviewLogsPipelinesHandler)).Methods(http.MethodPost)
	subRouter.HandleFunc("/pipelines/{version}", am.ViewAccess(aH.ListLogsPipelinesHandler)).Methods(http.MethodGet)
	subRouter.HandleFunc("/pipelines", am.EditAccess(aH.CreateLogsPipeline)).Methods(http.MethodPost)
}

// mountLogsLivetail registers live logs on the shared /v1/o11y subrouter owned
// by RegisterQueryRangeV3Routes. 1 route.
//
// It is NOT on the /v1/o11y/logs subrouter above: that subrouter is registered
// first and has no /livetail child, so gorilla/mux falls through to the
// /v1/o11y prefix route and this registration wins. Moving it under mountLogs
// would change nothing for callers but would change which prefix route matches
// — left exactly where it was.
func (aH *APIHandler) mountLogsLivetail(subRouter *mux.Router, am *middleware.AuthZ) {
	subRouter.HandleFunc("/logs/livetail", am.ViewAccess(aH.O11y.Handlers.QuerierHandler.QueryRawStream)).Methods(http.MethodGet)
}
