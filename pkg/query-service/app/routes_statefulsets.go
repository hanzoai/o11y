package app

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/hanzoai/o11y/pkg/http/middleware"
)

// mountStatefulsets registers k8s statefulset infra-metrics: attribute
// discovery + list. 3 routes on its own /v1/o11y/statefulsets subrouter.
//
// All three are ALSO declared as typed ops at the module's mount seam
// (infra.go in the repo root), which carries them into the composed document,
// the SDK, the CLI and the agent surface. That is a second DISPATCH, never a
// second implementation: the ops answer by handing the call back to this
// router, so the handlers below stay the one place the reads are performed,
// and the ViewAccess gate keeps running exactly here.
func (aH *APIHandler) mountStatefulsets(router *mux.Router, am *middleware.AuthZ) {
	statefulsetsSubRouter := router.PathPrefix("/v1/o11y/statefulsets").Subrouter()
	statefulsetsSubRouter.HandleFunc("/attribute_keys", am.ViewAccess(aH.getStatefulSetAttributeKeys)).Methods(http.MethodGet)
	statefulsetsSubRouter.HandleFunc("/attribute_values", am.ViewAccess(aH.getStatefulSetAttributeValues)).Methods(http.MethodGet)
	statefulsetsSubRouter.HandleFunc("/list", am.ViewAccess(aH.getStatefulSetList)).Methods(http.MethodPost)
}
