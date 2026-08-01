package app

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/hanzoai/o11y/pkg/http/middleware"
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
func (aH *APIHandler) mountThirdPartyApis(router *mux.Router, am *middleware.AuthZ) {
	thirdPartyApiRouter := router.PathPrefix("/v1/o11y/third-party-apis").Subrouter()

	// Domain Overview route
	overviewRouter := thirdPartyApiRouter.PathPrefix("/overview").Subrouter()

	overviewRouter.HandleFunc("/list", am.ViewAccess(aH.getDomainList)).Methods(http.MethodPost)
	overviewRouter.HandleFunc("/domain", am.ViewAccess(aH.getDomainInfo)).Methods(http.MethodPost)
}
