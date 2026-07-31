package app

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/hanzoai/o11y/pkg/http/middleware"
)

// mountSettings registers tenant settings: retention TTL and apdex. 4 routes.
// Write is AdminAccess, read is ViewAccess for both.
func (aH *APIHandler) mountSettings(router *mux.Router, am *middleware.AuthZ) {
	router.HandleFunc("/v1/o11y/settings/ttl", am.AdminAccess(aH.setCustomRetentionTTL)).Methods(http.MethodPost)
	router.HandleFunc("/v1/o11y/settings/ttl", am.ViewAccess(aH.getCustomRetentionTTL)).Methods(http.MethodGet)

	router.HandleFunc("/v1/o11y/settings/apdex", am.AdminAccess(aH.O11y.Handlers.Apdex.Set)).Methods(http.MethodPost)
	router.HandleFunc("/v1/o11y/settings/apdex", am.ViewAccess(aH.O11y.Handlers.Apdex.Get)).Methods(http.MethodGet)
}
