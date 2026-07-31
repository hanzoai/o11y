package app

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/hanzoai/o11y/pkg/http/middleware"
)

// mountExplorer registers saved explorer views (list/create/get/update/delete).
// 5 routes.
func (aH *APIHandler) mountExplorer(router *mux.Router, am *middleware.AuthZ) {
	router.HandleFunc("/v1/o11y/explorer/views", am.ViewAccess(aH.O11y.Handlers.SavedView.List)).Methods(http.MethodGet)
	router.HandleFunc("/v1/o11y/explorer/views", am.EditAccess(aH.O11y.Handlers.SavedView.Create)).Methods(http.MethodPost)
	router.HandleFunc("/v1/o11y/explorer/views/{viewId}", am.ViewAccess(aH.O11y.Handlers.SavedView.Get)).Methods(http.MethodGet)
	router.HandleFunc("/v1/o11y/explorer/views/{viewId}", am.EditAccess(aH.O11y.Handlers.SavedView.Update)).Methods(http.MethodPut)
	router.HandleFunc("/v1/o11y/explorer/views/{viewId}", am.EditAccess(aH.O11y.Handlers.SavedView.Delete)).Methods(http.MethodDelete)
}
