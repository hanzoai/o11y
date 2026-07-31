package app

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/hanzoai/o11y/pkg/http/middleware"
)

// mountPods registers k8s pod infra-metrics: attribute discovery + list.
// 3 routes on its own /v1/o11y/pods subrouter.
func (aH *APIHandler) mountPods(router *mux.Router, am *middleware.AuthZ) {
	podsSubRouter := router.PathPrefix("/v1/o11y/pods").Subrouter()
	podsSubRouter.HandleFunc("/attribute_keys", am.ViewAccess(aH.getPodAttributeKeys)).Methods(http.MethodGet)
	podsSubRouter.HandleFunc("/attribute_values", am.ViewAccess(aH.getPodAttributeValues)).Methods(http.MethodGet)
	podsSubRouter.HandleFunc("/list", am.ViewAccess(aH.getPodList)).Methods(http.MethodPost)
}
