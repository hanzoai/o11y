package app

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/hanzoai/o11y/pkg/http/middleware"
	"github.com/hanzoai/o11y/pkg/http/render"
)

// mountLicenses registers the licensing surface. 2 routes.
// The list is intentionally an empty set — there is no enterprise edition in
// this build — while activation is delegated to the licensing API.
func (aH *APIHandler) mountLicenses(router *mux.Router, am *middleware.AuthZ) {
	router.HandleFunc("/v1/o11y/licenses", am.ViewAccess(func(rw http.ResponseWriter, req *http.Request) {
		render.Success(rw, http.StatusOK, []any{})
	})).Methods(http.MethodGet)
	router.HandleFunc("/v1/o11y/licenses/active", am.ViewAccess(func(rw http.ResponseWriter, req *http.Request) {
		aH.LicensingAPI.Activate(rw, req)
	})).Methods(http.MethodGet)
}
