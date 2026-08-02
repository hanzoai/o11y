package app

import (
	"github.com/hanzoai/o11y/pkg/http/middleware"
	"github.com/hanzoai/o11y/pkg/http/routing"
)

// mountSettings registers tenant settings: retention TTL and apdex. 4 routes.
// Write is AdminAccess, read is ViewAccess for both.
//
// TWO DISPATCHES, ONE IMPLEMENTATION. These registrations stay: the standalone
// server has no native router to register an op on. The composed binary reaches
// the SAME handlers through the typed ops in the repo root's platform.go
// (setRetention, retention, setApdex, apdex), which relay here — so the gates
// named below stay the one place access is decided, and deleting either half
// drops one of the two deployments.
func (aH *APIHandler) mountSettings(router routing.Router, am *middleware.AuthZ) {
	router.Post("/v1/o11y/settings/ttl", am.AdminAccess(aH.setCustomRetentionTTL))
	router.Get("/v1/o11y/settings/ttl", am.ViewAccess(aH.getCustomRetentionTTL))

	router.Post("/v1/o11y/settings/apdex", am.AdminAccess(aH.O11y.Handlers.Apdex.Set))
	router.Get("/v1/o11y/settings/apdex", am.ViewAccess(aH.O11y.Handlers.Apdex.Get))
}
