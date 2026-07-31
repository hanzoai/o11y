package app

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/hanzoai/o11y/pkg/http/middleware"
)

// mountNodes registers k8s node infra-metrics: attribute discovery + list.
// 3 routes on its own /v1/o11y/nodes subrouter.
func (aH *APIHandler) mountNodes(router *mux.Router, am *middleware.AuthZ) {
	nodesSubRouter := router.PathPrefix("/v1/o11y/nodes").Subrouter()
	nodesSubRouter.HandleFunc("/attribute_keys", am.ViewAccess(aH.getNodeAttributeKeys)).Methods(http.MethodGet)
	nodesSubRouter.HandleFunc("/attribute_values", am.ViewAccess(aH.getNodeAttributeValues)).Methods(http.MethodGet)
	nodesSubRouter.HandleFunc("/list", am.ViewAccess(aH.getNodeList)).Methods(http.MethodPost)
}
