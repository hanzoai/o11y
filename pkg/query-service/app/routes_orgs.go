package app

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/hanzoai/o11y/pkg/http/middleware"
)

// mountOrgs registers the current org's quick filters. 3 routes.
// Read is ViewAccess, the bulk update is AdminAccess.
func (aH *APIHandler) mountOrgs(router *mux.Router, am *middleware.AuthZ) {
	router.HandleFunc("/v1/o11y/orgs/me/filters", am.ViewAccess(aH.O11y.Handlers.QuickFilter.GetQuickFilters)).Methods(http.MethodGet)
	router.HandleFunc("/v1/o11y/orgs/me/filters/{signal}", am.ViewAccess(aH.O11y.Handlers.QuickFilter.GetSignalFilters)).Methods(http.MethodGet)
	router.HandleFunc("/v1/o11y/orgs/me/filters", am.AdminAccess(aH.O11y.Handlers.QuickFilter.UpdateQuickFilters)).Methods(http.MethodPut)
}
