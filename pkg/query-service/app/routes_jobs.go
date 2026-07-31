package app

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/hanzoai/o11y/pkg/http/middleware"
)

// mountJobs registers k8s job infra-metrics: attribute discovery + list.
// 3 routes on its own /v1/o11y/jobs subrouter.
func (aH *APIHandler) mountJobs(router *mux.Router, am *middleware.AuthZ) {
	jobsSubRouter := router.PathPrefix("/v1/o11y/jobs").Subrouter()
	jobsSubRouter.HandleFunc("/attribute_keys", am.ViewAccess(aH.getJobAttributeKeys)).Methods(http.MethodGet)
	jobsSubRouter.HandleFunc("/attribute_values", am.ViewAccess(aH.getJobAttributeValues)).Methods(http.MethodGet)
	jobsSubRouter.HandleFunc("/list", am.ViewAccess(aH.getJobList)).Methods(http.MethodPost)
}
