package app

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/hanzoai/o11y/pkg/http/middleware"
)

// mountIntegrations registers the integrations catalog and install lifecycle.
// 5 routes on its own /v1/o11y/integrations subrouter.
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
