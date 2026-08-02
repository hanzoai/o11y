package app

import (
	"github.com/hanzoai/o11y/pkg/http/middleware"
	"github.com/hanzoai/o11y/pkg/http/routing"
)

// mountClusters registers k8s cluster infra-metrics: attribute discovery +
// list. 3 routes on its own /v1/o11y/clusters subrouter.
//
// All three are ALSO declared as typed ops at the module's mount seam
// (infra.go in the repo root), which carries them into the composed document,
// the SDK, the CLI and the agent surface. That is a second DISPATCH, never a
// second implementation: the ops answer by handing the call back to this
// router, so the handlers below stay the one place the reads are performed,
// and the ViewAccess gate keeps running exactly here.
func (aH *APIHandler) mountClusters(router routing.Router, am *middleware.AuthZ) {
	clustersSubRouter := router.Group("/v1/o11y/clusters")
	clustersSubRouter.Get("/attribute_keys", am.ViewAccess(aH.getClusterAttributeKeys))
	clustersSubRouter.Get("/attribute_values", am.ViewAccess(aH.getClusterAttributeValues))
	clustersSubRouter.Post("/list", am.ViewAccess(aH.getClusterList))
}
