package app

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/hanzoai/o11y/pkg/http/middleware"
)

// mountNamespaces registers k8s namespace infra-metrics: attribute discovery +
// list. 3 routes on its own /v1/o11y/namespaces subrouter.
func (aH *APIHandler) mountNamespaces(router *mux.Router, am *middleware.AuthZ) {
	namespacesSubRouter := router.PathPrefix("/v1/o11y/namespaces").Subrouter()
	namespacesSubRouter.HandleFunc("/attribute_keys", am.ViewAccess(aH.getNamespaceAttributeKeys)).Methods(http.MethodGet)
	namespacesSubRouter.HandleFunc("/attribute_values", am.ViewAccess(aH.getNamespaceAttributeValues)).Methods(http.MethodGet)
	namespacesSubRouter.HandleFunc("/list", am.ViewAccess(aH.getNamespaceList)).Methods(http.MethodPost)
}
