package app

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/hanzoai/o11y/pkg/http/middleware"
)

// mountRules registers alert-rule evaluation and rule state history. 5 routes.
//
// rules + rules/{id} (GET/POST/PUT/DELETE/PATCH) are NOT here: they are served
// by o11yapiserver/ruler.go (formerly /api/v2/rules). Highest version wins —
// the same handler the version-less /v1/o11y contract already resolved to
// before the flatten.
func (aH *APIHandler) mountRules(router *mux.Router, am *middleware.AuthZ) {
	router.HandleFunc("/v1/o11y/testRule", am.EditAccess(aH.testRule)).Methods(http.MethodPost)
	router.HandleFunc("/v1/o11y/rules/{id}/history/stats", am.ViewAccess(aH.getRuleStats)).Methods(http.MethodPost)
	router.HandleFunc("/v1/o11y/rules/{id}/history/timeline", am.ViewAccess(aH.getRuleStateHistory)).Methods(http.MethodPost)
	router.HandleFunc("/v1/o11y/rules/{id}/history/top_contributors", am.ViewAccess(aH.getRuleStateHistoryTopContributors)).Methods(http.MethodPost)
	router.HandleFunc("/v1/o11y/rules/{id}/history/overall_status", am.ViewAccess(aH.getOverallStateTransitions)).Methods(http.MethodPost)
}
