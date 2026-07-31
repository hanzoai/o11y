package app

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/hanzoai/o11y/pkg/http/middleware"
)

// mountDaemonsets registers k8s daemonset infra-metrics: attribute discovery +
// list. 3 routes on its own /v1/o11y/daemonsets subrouter.
func (aH *APIHandler) mountDaemonsets(router *mux.Router, am *middleware.AuthZ) {
	daemonsetsSubRouter := router.PathPrefix("/v1/o11y/daemonsets").Subrouter()
	daemonsetsSubRouter.HandleFunc("/attribute_keys", am.ViewAccess(aH.getDaemonSetAttributeKeys)).Methods(http.MethodGet)
	daemonsetsSubRouter.HandleFunc("/attribute_values", am.ViewAccess(aH.getDaemonSetAttributeValues)).Methods(http.MethodGet)
	daemonsetsSubRouter.HandleFunc("/list", am.ViewAccess(aH.getDaemonSetList)).Methods(http.MethodPost)
}
