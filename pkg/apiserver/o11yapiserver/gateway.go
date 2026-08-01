package o11yapiserver

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/hanzoai/o11y/pkg/http/handler"
	"github.com/hanzoai/o11y/pkg/types"
	"github.com/hanzoai/o11y/pkg/types/gatewaytypes"
)

// ALL of these routes are ALSO declared as typed ops at the module's mount seam
// (integrations.go in the repo root), which is what carries them into the
// composed document, the SDK, the CLI and the agent surface. That is a second
// DISPATCH, never a second implementation: the ops answer by handing the call
// to this router, so the handlers below stay the one place the work is
// performed — and the EditAccess gate declared here stays the one place access
// is decided. Both halves are needed — the ops serve the composed binary, this
// router serves the standalone process, which has no native router to register
// an op on — so deleting either drops one of the two deployments.
func (provider *provider) addGatewayRoutes(router *mux.Router) error {
	if err := router.Handle("/v1/o11y/gateway/ingestion_keys", handler.New(provider.authzMiddleware.EditAccess(provider.gatewayHandler.GetIngestionKeys), handler.OpenAPIDef{
		ID:                  "GetIngestionKeys",
		Tags:                []string{"gateway"},
		Summary:             "Get ingestion keys for workspace",
		Description:         "This endpoint returns the ingestion keys for a workspace",
		Request:             nil,
		RequestQuery:        new(gatewaytypes.IngestionKeysParams),
		RequestContentType:  "",
		Response:            new(gatewaytypes.GettableIngestionKeys),
		ResponseContentType: "application/json",
		SuccessStatusCode:   http.StatusOK,
		ErrorStatusCodes:    []int{},
		Deprecated:          false,
		SecuritySchemes:     newSecuritySchemes(types.RoleEditor),
	})).Methods(http.MethodGet).GetError(); err != nil {
		return err
	}

	if err := router.Handle("/v1/o11y/gateway/ingestion_keys/search", handler.New(provider.authzMiddleware.EditAccess(provider.gatewayHandler.SearchIngestionKeys), handler.OpenAPIDef{
		ID:                  "SearchIngestionKeys",
		Tags:                []string{"gateway"},
		Summary:             "Search ingestion keys for workspace",
		Description:         "This endpoint returns the ingestion keys for a workspace",
		Request:             nil,
		RequestQuery:        new(gatewaytypes.SearchIngestionKeysParams),
		RequestContentType:  "",
		Response:            new(gatewaytypes.GettableIngestionKeys),
		ResponseContentType: "application/json",
		SuccessStatusCode:   http.StatusOK,
		ErrorStatusCodes:    []int{},
		Deprecated:          false,
		SecuritySchemes:     newSecuritySchemes(types.RoleEditor),
	})).Methods(http.MethodGet).GetError(); err != nil {
		return err
	}

	if err := router.Handle("/v1/o11y/gateway/ingestion_keys", handler.New(provider.authzMiddleware.EditAccess(provider.gatewayHandler.CreateIngestionKey), handler.OpenAPIDef{
		ID:                  "CreateIngestionKey",
		Tags:                []string{"gateway"},
		Summary:             "Create ingestion key for workspace",
		Description:         "This endpoint creates an ingestion key for the workspace",
		Request:             new(gatewaytypes.PostableIngestionKey),
		RequestContentType:  "application/json",
		Response:            new(gatewaytypes.GettableCreatedIngestionKey),
		ResponseContentType: "application/json",
		SuccessStatusCode:   http.StatusCreated,
		ErrorStatusCodes:    []int{},
		Deprecated:          false,
		SecuritySchemes:     newSecuritySchemes(types.RoleEditor),
	})).Methods(http.MethodPost).GetError(); err != nil {
		return err
	}

	if err := router.Handle("/v1/o11y/gateway/ingestion_keys/{keyId}", handler.New(provider.authzMiddleware.EditAccess(provider.gatewayHandler.UpdateIngestionKey), handler.OpenAPIDef{
		ID:                  "UpdateIngestionKey",
		Tags:                []string{"gateway"},
		Summary:             "Update ingestion key for workspace",
		Description:         "This endpoint updates an ingestion key for the workspace",
		Request:             new(gatewaytypes.PostableIngestionKey),
		RequestContentType:  "application/json",
		Response:            nil,
		ResponseContentType: "",
		SuccessStatusCode:   http.StatusNoContent,
		ErrorStatusCodes:    []int{},
		Deprecated:          false,
		SecuritySchemes:     newSecuritySchemes(types.RoleEditor),
	})).Methods(http.MethodPatch).GetError(); err != nil {
		return err
	}

	if err := router.Handle("/v1/o11y/gateway/ingestion_keys/{keyId}", handler.New(provider.authzMiddleware.EditAccess(provider.gatewayHandler.DeleteIngestionKey), handler.OpenAPIDef{
		ID:                  "DeleteIngestionKey",
		Tags:                []string{"gateway"},
		Summary:             "Delete ingestion key for workspace",
		Description:         "This endpoint deletes an ingestion key for the workspace",
		Request:             nil,
		RequestContentType:  "",
		Response:            nil,
		ResponseContentType: "",
		SuccessStatusCode:   http.StatusNoContent,
		ErrorStatusCodes:    []int{},
		Deprecated:          false,
		SecuritySchemes:     newSecuritySchemes(types.RoleEditor),
	})).Methods(http.MethodDelete).GetError(); err != nil {
		return err
	}

	if err := router.Handle("/v1/o11y/gateway/ingestion_keys/{keyId}/limits", handler.New(provider.authzMiddleware.EditAccess(provider.gatewayHandler.CreateIngestionKeyLimit), handler.OpenAPIDef{
		ID:                  "CreateIngestionKeyLimit",
		Tags:                []string{"gateway"},
		Summary:             "Create limit for the ingestion key",
		Description:         "This endpoint creates an ingestion key limit",
		Request:             new(gatewaytypes.PostableIngestionKeyLimit),
		RequestContentType:  "application/json",
		Response:            new(gatewaytypes.GettableCreatedIngestionKeyLimit),
		ResponseContentType: "application/json",
		SuccessStatusCode:   http.StatusCreated,
		ErrorStatusCodes:    []int{},
		Deprecated:          false,
		SecuritySchemes:     newSecuritySchemes(types.RoleEditor),
	})).Methods(http.MethodPost).GetError(); err != nil {
		return err
	}

	if err := router.Handle("/v1/o11y/gateway/ingestion_keys/limits/{limitId}", handler.New(provider.authzMiddleware.EditAccess(provider.gatewayHandler.UpdateIngestionKeyLimit), handler.OpenAPIDef{
		ID:                  "UpdateIngestionKeyLimit",
		Tags:                []string{"gateway"},
		Summary:             "Update limit for the ingestion key",
		Description:         "This endpoint updates an ingestion key limit",
		Request:             new(gatewaytypes.UpdatableIngestionKeyLimit),
		RequestContentType:  "application/json",
		Response:            nil,
		ResponseContentType: "",
		SuccessStatusCode:   http.StatusNoContent,
		ErrorStatusCodes:    []int{},
		Deprecated:          false,
		SecuritySchemes:     newSecuritySchemes(types.RoleEditor),
	})).Methods(http.MethodPatch).GetError(); err != nil {
		return err
	}

	if err := router.Handle("/v1/o11y/gateway/ingestion_keys/limits/{limitId}", handler.New(provider.authzMiddleware.EditAccess(provider.gatewayHandler.DeleteIngestionKeyLimit), handler.OpenAPIDef{
		ID:                  "DeleteIngestionKeyLimit",
		Tags:                []string{"gateway"},
		Summary:             "Delete limit for the ingestion key",
		Description:         "This endpoint deletes an ingestion key limit",
		Request:             nil,
		RequestContentType:  "application/json",
		Response:            nil,
		ResponseContentType: "",
		SuccessStatusCode:   http.StatusNoContent,
		ErrorStatusCodes:    []int{},
		Deprecated:          false,
		SecuritySchemes:     newSecuritySchemes(types.RoleEditor),
	})).Methods(http.MethodDelete).GetError(); err != nil {
		return err
	}

	return nil
}
