package app

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/hanzoai/o11y/pkg/http/middleware"
)

// mountClusters registers k8s cluster infra-metrics: attribute discovery +
// list. 3 routes on its own /v1/o11y/clusters subrouter.
func (aH *APIHandler) mountClusters(router *mux.Router, am *middleware.AuthZ) {
	clustersSubRouter := router.PathPrefix("/v1/o11y/clusters").Subrouter()
	clustersSubRouter.HandleFunc("/attribute_keys", am.ViewAccess(aH.getClusterAttributeKeys)).Methods(http.MethodGet)
	clustersSubRouter.HandleFunc("/attribute_values", am.ViewAccess(aH.getClusterAttributeValues)).Methods(http.MethodGet)
	clustersSubRouter.HandleFunc("/list", am.ViewAccess(aH.getClusterList)).Methods(http.MethodPost)
}
