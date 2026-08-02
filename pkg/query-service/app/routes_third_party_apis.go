package app

import (
	"github.com/hanzoai/o11y/pkg/http/middleware"
	"github.com/hanzoai/o11y/pkg/http/routing"
)

// mountThirdPartyApis registers the third-party (external domain) API overview.
// 2 routes under /v1/o11y/third-party-apis/overview.
//
// TWO DISPATCHES, ONE IMPLEMENTATION. These registrations stay: the standalone
// server has no native router to register an op on. The composed binary reaches
// the SAME handlers through the typed ops in the repo root's apm.go (domainList,
// domainInfo), which relay here — deleting either half drops one of the two
// deployments. The ViewAccess gate on each route is unchanged: it runs here,
// one layer in.
func (aH *APIHandler) mountThirdPartyApis(router routing.Router, am *middleware.AuthZ) {
	thirdPartyApiRouter := router.Group("/v1/o11y/third-party-apis")

	// Domain Overview route
	overviewRouter := thirdPartyApiRouter.Group("/overview")

	overviewRouter.Post("/list", am.ViewAccess(aH.getDomainList))
	overviewRouter.Post("/domain", am.ViewAccess(aH.getDomainInfo))
}
