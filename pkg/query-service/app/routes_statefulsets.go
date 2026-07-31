package app

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/hanzoai/o11y/pkg/http/middleware"
)

// mountStatefulsets registers k8s statefulset infra-metrics: attribute
// discovery + list. 3 routes on its own /v1/o11y/statefulsets subrouter.
func (aH *APIHandler) mountStatefulsets(router *mux.Router, am *middleware.AuthZ) {
	statefulsetsSubRouter := router.PathPrefix("/v1/o11y/statefulsets").Subrouter()
	statefulsetsSubRouter.HandleFunc("/attribute_keys", am.ViewAccess(aH.getStatefulSetAttributeKeys)).Methods(http.MethodGet)
	statefulsetsSubRouter.HandleFunc("/attribute_values", am.ViewAccess(aH.getStatefulSetAttributeValues)).Methods(http.MethodGet)
	statefulsetsSubRouter.HandleFunc("/list", am.ViewAccess(aH.getStatefulSetList)).Methods(http.MethodPost)
}
