package app

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/hanzoai/o11y/pkg/http/middleware"
)

// mountPods registers k8s pod infra-metrics: attribute discovery + list.
// 3 routes on its own /v1/o11y/pods subrouter.
//
// All three are ALSO declared as typed ops at the module's mount seam
// (infra.go in the repo root), which carries them into the composed document,
// the SDK, the CLI and the agent surface. That is a second DISPATCH, never a
// second implementation: the ops answer by handing the call back to this
// router, so the handlers below stay the one place the reads are performed,
// and the ViewAccess gate keeps running exactly here.
func (aH *APIHandler) mountPods(router *mux.Router, am *middleware.AuthZ) {
	podsSubRouter := router.PathPrefix("/v1/o11y/pods").Subrouter()
	podsSubRouter.HandleFunc("/attribute_keys", am.ViewAccess(aH.getPodAttributeKeys)).Methods(http.MethodGet)
	podsSubRouter.HandleFunc("/attribute_values", am.ViewAccess(aH.getPodAttributeValues)).Methods(http.MethodGet)
	podsSubRouter.HandleFunc("/list", am.ViewAccess(aH.getPodList)).Methods(http.MethodPost)
}
