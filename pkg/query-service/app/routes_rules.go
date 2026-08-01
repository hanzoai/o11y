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
// ALL of these routes are ALSO declared as typed ops at the module's mount seam
// (rulesalerts.go in the repo root) — the v1 history reads and the legacy
// /testRule — which is what carries them into the composed document, the SDK,
// the CLI and the agent surface. That is a second DISPATCH, never a second
// implementation: the ops answer by handing the call back to this router, so
// the handlers below stay the one place the work is performed — and the gates
// declared here (ViewAccess, EditAccess) stay the one place access is decided.
// Both halves are needed — the ops serve the composed binary, this router
// serves the standalone process, which has no native router to register an op
// on — so deleting either drops one of the two deployments.
func (aH *APIHandler) mountRules(router *mux.Router, am *middleware.AuthZ) {
	router.HandleFunc("/v1/o11y/testRule", am.EditAccess(aH.testRule)).Methods(http.MethodPost)
	router.HandleFunc("/v1/o11y/rules/{id}/history/stats", am.ViewAccess(aH.getRuleStats)).Methods(http.MethodPost)
	router.HandleFunc("/v1/o11y/rules/{id}/history/timeline", am.ViewAccess(aH.getRuleStateHistory)).Methods(http.MethodPost)
	router.HandleFunc("/v1/o11y/rules/{id}/history/top_contributors", am.ViewAccess(aH.getRuleStateHistoryTopContributors)).Methods(http.MethodPost)
	router.HandleFunc("/v1/o11y/rules/{id}/history/overall_status", am.ViewAccess(aH.getOverallStateTransitions)).Methods(http.MethodPost)
}
