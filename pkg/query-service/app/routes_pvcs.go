package app

import (
	"github.com/hanzoai/o11y/pkg/http/middleware"
	"github.com/hanzoai/o11y/pkg/http/routing"
)

// mountPvcs registers k8s persistent-volume-claim infra-metrics: attribute
// discovery + list. 3 routes on its own /v1/o11y/pvcs subrouter.
//
// All three are ALSO declared as typed ops at the module's mount seam
// (infra.go in the repo root), which carries them into the composed document,
// the SDK, the CLI and the agent surface. That is a second DISPATCH, never a
// second implementation: the ops answer by handing the call back to this
// router, so the handlers below stay the one place the reads are performed,
// and the ViewAccess gate keeps running exactly here.
func (aH *APIHandler) mountPvcs(router routing.Router, am *middleware.AuthZ) {
	pvcsSubRouter := router.Group("/v1/o11y/pvcs")
	pvcsSubRouter.Get("/attribute_keys", am.ViewAccess(aH.getPvcAttributeKeys))
	pvcsSubRouter.Get("/attribute_values", am.ViewAccess(aH.getPvcAttributeValues))
	pvcsSubRouter.Post("/list", am.ViewAccess(aH.getPvcList))
}
