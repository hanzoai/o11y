package app

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/hanzoai/o11y/pkg/http/middleware"
)

// mountDeployments registers k8s deployment infra-metrics: attribute discovery
// + list. 3 routes on its own /v1/o11y/deployments subrouter.
func (aH *APIHandler) mountDeployments(router *mux.Router, am *middleware.AuthZ) {
	deploymentsSubRouter := router.PathPrefix("/v1/o11y/deployments").Subrouter()
	deploymentsSubRouter.HandleFunc("/attribute_keys", am.ViewAccess(aH.getDeploymentAttributeKeys)).Methods(http.MethodGet)
	deploymentsSubRouter.HandleFunc("/attribute_values", am.ViewAccess(aH.getDeploymentAttributeValues)).Methods(http.MethodGet)
	deploymentsSubRouter.HandleFunc("/list", am.ViewAccess(aH.getDeploymentList)).Methods(http.MethodPost)
}
