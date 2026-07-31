package app

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/hanzoai/o11y/pkg/http/middleware"
)

// mountServices registers the APM service catalog: the /services collection and
// the singular /service/* operation breakdowns. 5 routes.
//
// /services and /service are two literal prefixes of one resource, kept in one
// file on purpose — a routes_service.go next to a routes_services.go is a
// one-character trap.
func (aH *APIHandler) mountServices(router *mux.Router, am *middleware.AuthZ) {
	router.HandleFunc("/v1/o11y/services", am.ViewAccess(aH.O11y.Handlers.Services.Get)).Methods(http.MethodPost)
	router.HandleFunc("/v1/o11y/services/list", am.ViewAccess(aH.getServicesList)).Methods(http.MethodGet)

	router.HandleFunc("/v1/o11y/service/top_operations", am.ViewAccess(aH.O11y.Handlers.Services.GetTopOperations)).Methods(http.MethodPost)
	router.HandleFunc("/v1/o11y/service/top_level_operations", am.ViewAccess(aH.getServicesTopLevelOps)).Methods(http.MethodPost)

	router.HandleFunc("/v1/o11y/service/entry_point_operations", am.ViewAccess(aH.O11y.Handlers.Services.GetEntryPointOperations)).Methods(http.MethodPost)
}
