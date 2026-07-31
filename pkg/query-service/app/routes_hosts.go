package app

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/hanzoai/o11y/pkg/http/middleware"
)

// mountHosts registers host infra-metrics: attribute discovery + list.
// 3 routes on its own /v1/o11y/hosts subrouter.
func (aH *APIHandler) mountHosts(router *mux.Router, am *middleware.AuthZ) {
	hostsSubRouter := router.PathPrefix("/v1/o11y/hosts").Subrouter()
	hostsSubRouter.HandleFunc("/attribute_keys", am.ViewAccess(aH.getHostAttributeKeys)).Methods(http.MethodGet)
	hostsSubRouter.HandleFunc("/attribute_values", am.ViewAccess(aH.getHostAttributeValues)).Methods(http.MethodGet)
	hostsSubRouter.HandleFunc("/list", am.ViewAccess(aH.getHostList)).Methods(http.MethodPost)
}
