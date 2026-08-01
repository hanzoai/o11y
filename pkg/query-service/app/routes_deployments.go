package app

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/hanzoai/o11y/pkg/http/middleware"
)

// mountDeployments registers k8s deployment infra-metrics: attribute discovery
// + list. 3 routes on its own /v1/o11y/deployments subrouter.
//
// All three are ALSO declared as typed ops at the module's mount seam
// (infra.go in the repo root), which carries them into the composed document,
// the SDK, the CLI and the agent surface. That is a second DISPATCH, never a
// second implementation: the ops answer by handing the call back to this
// router, so the handlers below stay the one place the reads are performed,
// and the ViewAccess gate keeps running exactly here.
func (aH *APIHandler) mountDeployments(router *mux.Router, am *middleware.AuthZ) {
	deploymentsSubRouter := router.PathPrefix("/v1/o11y/deployments").Subrouter()
	deploymentsSubRouter.HandleFunc("/attribute_keys", am.ViewAccess(aH.getDeploymentAttributeKeys)).Methods(http.MethodGet)
	deploymentsSubRouter.HandleFunc("/attribute_values", am.ViewAccess(aH.getDeploymentAttributeValues)).Methods(http.MethodGet)
	deploymentsSubRouter.HandleFunc("/list", am.ViewAccess(aH.getDeploymentList)).Methods(http.MethodPost)
}
