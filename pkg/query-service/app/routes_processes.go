package app

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/hanzoai/o11y/pkg/http/middleware"
)

// mountProcesses registers process infra-metrics: attribute discovery + list.
// 3 routes on its own /v1/o11y/processes subrouter.
func (aH *APIHandler) mountProcesses(router *mux.Router, am *middleware.AuthZ) {
	processesSubRouter := router.PathPrefix("/v1/o11y/processes").Subrouter()
	processesSubRouter.HandleFunc("/attribute_keys", am.ViewAccess(aH.getProcessAttributeKeys)).Methods(http.MethodGet)
	processesSubRouter.HandleFunc("/attribute_values", am.ViewAccess(aH.getProcessAttributeValues)).Methods(http.MethodGet)
	processesSubRouter.HandleFunc("/list", am.ViewAccess(aH.getProcessList)).Methods(http.MethodPost)
}
