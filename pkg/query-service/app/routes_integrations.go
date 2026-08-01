package app

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/hanzoai/o11y/pkg/http/middleware"
)

// mountIntegrations registers the integrations catalog and install lifecycle.
// 5 routes on its own /v1/o11y/integrations subrouter.
//
// These 5 routes are ALSO declared as typed ops at the module's mount seam
// (integrations.go in the repo root), which is what carries them into the
// composed document, the SDK, the CLI and the agent surface. That is a second
// DISPATCH, never a second implementation: the ops answer by handing the call
// to this router, so the handlers here stay the one place the work is performed
// — and the ViewAccess gate stays the one place access is decided. Both halves
// are needed — the ops serve the composed binary, this router serves the
// standalone process, which has no native router to register an op on — so
// deleting either drops one of the two deployments.
//
// ORDER IS LOAD-BEARING: /install and /uninstall are registered before
// /{integrationId}, and /{integrationId}/connection_status before
// /{integrationId}. Preserved verbatim.
func (aH *APIHandler) mountIntegrations(router *mux.Router, am *middleware.AuthZ) {
	subRouter := router.PathPrefix("/v1/o11y/integrations").Subrouter()

	subRouter.HandleFunc(
		"/install", am.ViewAccess(aH.InstallIntegration),
	).Methods(http.MethodPost)

	subRouter.HandleFunc(
		"/uninstall", am.ViewAccess(aH.UninstallIntegration),
	).Methods(http.MethodPost)

	// Used for polling for status in v0
	subRouter.HandleFunc(
		"/{integrationId}/connection_status", am.ViewAccess(aH.GetIntegrationConnectionStatus),
	).Methods(http.MethodGet)

	subRouter.HandleFunc(
		"/{integrationId}", am.ViewAccess(aH.GetIntegration),
	).Methods(http.MethodGet)

	subRouter.HandleFunc(
		"", am.ViewAccess(aH.ListIntegrations),
	).Methods(http.MethodGet)
}
