package app

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/hanzoai/o11y/pkg/http/middleware"
)

// mountThirdPartyApis registers the third-party (external domain) API overview.
// 2 routes under /v1/o11y/third-party-apis/overview.
func (aH *APIHandler) mountThirdPartyApis(router *mux.Router, am *middleware.AuthZ) {
	thirdPartyApiRouter := router.PathPrefix("/v1/o11y/third-party-apis").Subrouter()

	// Domain Overview route
	overviewRouter := thirdPartyApiRouter.PathPrefix("/overview").Subrouter()

	overviewRouter.HandleFunc("/list", am.ViewAccess(aH.getDomainList)).Methods(http.MethodPost)
	overviewRouter.HandleFunc("/domain", am.ViewAccess(aH.getDomainInfo)).Methods(http.MethodPost)
}
