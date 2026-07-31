package app

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/hanzoai/o11y/pkg/http/middleware"
)

// mountErrors registers the exceptions/errors surface: list, count, and the
// three single-error lookups. 5 routes.
//
// These are five distinct top-level path segments (listErrors, countErrors,
// errorFromErrorID, errorFromGroupID, nextPrevErrorIDs) but one resource; they
// share a file so the exceptions surface converts as a unit.
func (aH *APIHandler) mountErrors(router *mux.Router, am *middleware.AuthZ) {
	router.HandleFunc("/v1/o11y/listErrors", am.ViewAccess(aH.listErrors)).Methods(http.MethodPost)
	router.HandleFunc("/v1/o11y/countErrors", am.ViewAccess(aH.countErrors)).Methods(http.MethodPost)
	router.HandleFunc("/v1/o11y/errorFromErrorID", am.ViewAccess(aH.getErrorFromErrorID)).Methods(http.MethodGet)
	router.HandleFunc("/v1/o11y/errorFromGroupID", am.ViewAccess(aH.getErrorFromGroupID)).Methods(http.MethodGet)
	router.HandleFunc("/v1/o11y/nextPrevErrorIDs", am.ViewAccess(aH.getNextPrevErrorIDs)).Methods(http.MethodGet)
}
