package app

import (
	"github.com/hanzoai/o11y/pkg/http/middleware"
	"github.com/hanzoai/o11y/pkg/http/routing"
)

// DUAL DISPATCH. These registrations stay: the standalone server reaches them
// directly, and the composed binary reaches the SAME handlers through the typed
// saved-view ops in the repo root's querycore.go, which relay here. Deleting
// either half drops one of the two deployments.

// mountExplorer registers saved explorer views (list/create/get/update/delete).
// 5 routes.
func (aH *APIHandler) mountExplorer(router routing.Router, am *middleware.AuthZ) {
	router.Get("/v1/o11y/explorer/views", am.ViewAccess(aH.O11y.Handlers.SavedView.List))
	router.Post("/v1/o11y/explorer/views", am.EditAccess(aH.O11y.Handlers.SavedView.Create))
	router.Get("/v1/o11y/explorer/views/{viewId}", am.ViewAccess(aH.O11y.Handlers.SavedView.Get))
	router.Put("/v1/o11y/explorer/views/{viewId}", am.EditAccess(aH.O11y.Handlers.SavedView.Update))
	router.Delete("/v1/o11y/explorer/views/{viewId}", am.EditAccess(aH.O11y.Handlers.SavedView.Delete))
}
