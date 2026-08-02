package o11yapiserver

import (
	"net/http"

	"github.com/hanzoai/o11y/pkg/http/handler"
	"github.com/hanzoai/o11y/pkg/http/routing"
	"github.com/hanzoai/o11y/pkg/types"
	"github.com/hanzoai/o11y/pkg/types/telemetrytypes"
)

// DUAL DISPATCH. These registrations stay: the standalone server reaches them
// directly, and the composed binary reaches the SAME handlers through the typed
// field-catalog ops in the repo root's querycore.go (fieldKeys, fieldValues),
// which relay here. Deleting either half drops one of the two deployments.
func (provider *provider) addFieldsRoutes(router routing.Router) {
	router.Get("/v1/o11y/fields/keys", handler.New(provider.authzMiddleware.ViewAccess(provider.fieldsHandler.GetFieldsKeys), handler.OpenAPIDef{
		ID:                  "GetFieldsKeys",
		Tags:                []string{"fields"},
		Summary:             "Get field keys",
		Description:         "This endpoint returns field keys",
		Request:             nil,
		RequestQuery:        new(telemetrytypes.PostableFieldKeysParams),
		RequestContentType:  "",
		Response:            new(telemetrytypes.GettableFieldKeys),
		ResponseContentType: "application/json",
		SuccessStatusCode:   http.StatusOK,
		ErrorStatusCodes:    []int{},
		Deprecated:          false,
		SecuritySchemes:     newSecuritySchemes(types.RoleViewer),
	}))

	router.Get("/v1/o11y/fields/values", handler.New(provider.authzMiddleware.ViewAccess(provider.fieldsHandler.GetFieldsValues), handler.OpenAPIDef{
		ID:                  "GetFieldsValues",
		Tags:                []string{"fields"},
		Summary:             "Get field values",
		Description:         "This endpoint returns field values",
		Request:             nil,
		RequestQuery:        new(telemetrytypes.PostableFieldValueParams),
		RequestContentType:  "",
		Response:            new(telemetrytypes.GettableFieldValues),
		ResponseContentType: "application/json",
		SuccessStatusCode:   http.StatusOK,
		ErrorStatusCodes:    []int{},
		Deprecated:          false,
		SecuritySchemes:     newSecuritySchemes(types.RoleViewer),
	}))
}
