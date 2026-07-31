package app

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/hanzoai/o11y/pkg/http/middleware"
)

// mountPvcs registers k8s persistent-volume-claim infra-metrics: attribute
// discovery + list. 3 routes on its own /v1/o11y/pvcs subrouter.
func (aH *APIHandler) mountPvcs(router *mux.Router, am *middleware.AuthZ) {
	pvcsSubRouter := router.PathPrefix("/v1/o11y/pvcs").Subrouter()
	pvcsSubRouter.HandleFunc("/attribute_keys", am.ViewAccess(aH.getPvcAttributeKeys)).Methods(http.MethodGet)
	pvcsSubRouter.HandleFunc("/attribute_values", am.ViewAccess(aH.getPvcAttributeValues)).Methods(http.MethodGet)
	pvcsSubRouter.HandleFunc("/list", am.ViewAccess(aH.getPvcList)).Methods(http.MethodPost)
}
