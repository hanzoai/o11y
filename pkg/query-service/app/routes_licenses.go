package app

import (
	"net/http"

	"github.com/hanzoai/o11y/pkg/http/middleware"
	"github.com/hanzoai/o11y/pkg/http/render"
	"github.com/hanzoai/o11y/pkg/http/routing"
)

// mountLicenses registers the licensing surface. 2 routes.
// The list is intentionally an empty set — there is no enterprise edition in
// this build — while activation is delegated to the licensing API.
//
// TWO DISPATCHES, ONE IMPLEMENTATION. These registrations stay: the standalone
// server has no native router to register an op on. The composed binary reaches
// the SAME handlers through the typed ops in the repo root's platform.go
// (licenses, activateLicense), which relay here — so the ViewAccess gates named
// below stay the one place access is decided, and deleting either half drops
// one of the two deployments.
func (aH *APIHandler) mountLicenses(router routing.Router, am *middleware.AuthZ) {
	router.Get("/v1/o11y/licenses", am.ViewAccess(func(rw http.ResponseWriter, req *http.Request) {
		render.Success(rw, http.StatusOK, []any{})
	}))
	router.Get("/v1/o11y/licenses/active", am.ViewAccess(func(rw http.ResponseWriter, req *http.Request) {
		aH.LicensingAPI.Activate(rw, req)
	}))
}
