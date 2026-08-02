package app

import (
	"github.com/hanzoai/o11y/pkg/http/middleware"
	"github.com/hanzoai/o11y/pkg/http/routing"
)

// mountErrors registers the exceptions/errors surface: list, count, and the
// three single-error lookups. 5 routes.
//
// These are five distinct top-level path segments (listErrors, countErrors,
// errorFromErrorID, errorFromGroupID, nextPrevErrorIDs) but one resource; they
// share a file so the exceptions surface converts as a unit.
//
// All five are ALSO typed ops at the module's mount seam (sentryerrors.go, on the
// /v1/o11y group), which is what carries them into the document, the SDK, the CLI
// and the agent surface. That is a second DISPATCH, never a second implementation:
// each op hands its call to THIS router, so the handlers below stay the one place
// the reads are performed — deleting either half drops one of the two deployments.
func (aH *APIHandler) mountErrors(router routing.Router, am *middleware.AuthZ) {
	router.Post("/v1/o11y/listErrors", am.ViewAccess(aH.listErrors))
	router.Post("/v1/o11y/countErrors", am.ViewAccess(aH.countErrors))
	router.Get("/v1/o11y/errorFromErrorID", am.ViewAccess(aH.getErrorFromErrorID))
	router.Get("/v1/o11y/errorFromGroupID", am.ViewAccess(aH.getErrorFromGroupID))
	router.Get("/v1/o11y/nextPrevErrorIDs", am.ViewAccess(aH.getNextPrevErrorIDs))
}
