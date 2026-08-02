package app

import (
	"github.com/hanzoai/o11y/pkg/http/middleware"
	"github.com/hanzoai/o11y/pkg/http/routing"
)

// mountOrgs registers the current org's quick filters. 3 routes.
// Read is ViewAccess, the bulk update is AdminAccess.
//
// ALL three are ALSO declared as typed ops at the module's mount seam
// (identity.go in the repo root) — a second DISPATCH onto this router, never a
// second implementation; the gates named above stay the one place access is
// decided. The ops serve the composed binary, this router the standalone
// process.
func (aH *APIHandler) mountOrgs(router routing.Router, am *middleware.AuthZ) {
	router.Get("/v1/o11y/orgs/me/filters", am.ViewAccess(aH.O11y.Handlers.QuickFilter.GetQuickFilters))
	router.Get("/v1/o11y/orgs/me/filters/{signal}", am.ViewAccess(aH.O11y.Handlers.QuickFilter.GetSignalFilters))
	router.Put("/v1/o11y/orgs/me/filters", am.AdminAccess(aH.O11y.Handlers.QuickFilter.UpdateQuickFilters))
}
