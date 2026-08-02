package o11yapiserver

import (
	"net/http"

	"github.com/hanzoai/o11y/pkg/http/handler"
	"github.com/hanzoai/o11y/pkg/http/routing"
	"github.com/hanzoai/o11y/pkg/types"
	"github.com/hanzoai/o11y/pkg/types/preferencetypes"
)

// ALL of these routes are ALSO declared as typed ops at the module's mount seam
// (identity.go in the repo root) — a second DISPATCH onto this router, never a
// second implementation; the ViewAccess/AdminAccess gates below stay the one
// place access is decided. The ops serve the composed binary, this router the
// standalone process.
func (provider *provider) addPreferenceRoutes(router routing.Router) {
	router.Get("/v1/o11y/user/preferences", handler.New(provider.authzMiddleware.ViewAccess(provider.preferenceHandler.ListByUser), handler.OpenAPIDef{
		ID:                  "ListUserPreferences",
		Tags:                []string{"preferences"},
		Summary:             "List user preferences",
		Description:         "This endpoint lists all user preferences",
		Request:             nil,
		RequestContentType:  "",
		Response:            make([]*preferencetypes.Preference, 0),
		ResponseContentType: "application/json",
		SuccessStatusCode:   http.StatusOK,
		ErrorStatusCodes:    []int{},
		Deprecated:          false,
		SecuritySchemes:     newSecuritySchemes(types.RoleViewer),
	}))

	router.Get("/v1/o11y/user/preferences/{name}", handler.New(provider.authzMiddleware.ViewAccess(provider.preferenceHandler.GetByUser), handler.OpenAPIDef{
		ID:                  "GetUserPreference",
		Tags:                []string{"preferences"},
		Summary:             "Get user preference",
		Description:         "This endpoint returns the user preference by name",
		Request:             nil,
		RequestContentType:  "",
		Response:            new(preferencetypes.Preference),
		ResponseContentType: "application/json",
		SuccessStatusCode:   http.StatusOK,
		ErrorStatusCodes:    []int{http.StatusNotFound},
		Deprecated:          false,
		SecuritySchemes:     newSecuritySchemes(types.RoleViewer),
	}))

	router.Put("/v1/o11y/user/preferences/{name}", handler.New(provider.authzMiddleware.ViewAccess(provider.preferenceHandler.UpdateByUser), handler.OpenAPIDef{
		ID:                  "UpdateUserPreference",
		Tags:                []string{"preferences"},
		Summary:             "Update user preference",
		Description:         "This endpoint updates the user preference by name",
		Request:             new(preferencetypes.UpdatablePreference),
		RequestContentType:  "application/json",
		Response:            nil,
		ResponseContentType: "",
		SuccessStatusCode:   http.StatusNoContent,
		ErrorStatusCodes:    []int{http.StatusBadRequest, http.StatusNotFound},
		Deprecated:          false,
		SecuritySchemes:     newSecuritySchemes(types.RoleViewer),
	}))

	router.Get("/v1/o11y/org/preferences", handler.New(provider.authzMiddleware.AdminAccess(provider.preferenceHandler.ListByOrg), handler.OpenAPIDef{
		ID:                  "ListOrgPreferences",
		Tags:                []string{"preferences"},
		Summary:             "List org preferences",
		Description:         "This endpoint lists all org preferences",
		Request:             nil,
		RequestContentType:  "",
		Response:            make([]*preferencetypes.Preference, 0),
		ResponseContentType: "application/json",
		SuccessStatusCode:   http.StatusOK,
		ErrorStatusCodes:    []int{},
		Deprecated:          false,
		SecuritySchemes:     newSecuritySchemes(types.RoleAdmin),
	}))

	router.Get("/v1/o11y/org/preferences/{name}", handler.New(provider.authzMiddleware.AdminAccess(provider.preferenceHandler.GetByOrg), handler.OpenAPIDef{
		ID:                  "GetOrgPreference",
		Tags:                []string{"preferences"},
		Summary:             "Get org preference",
		Description:         "This endpoint returns the org preference by name",
		Request:             nil,
		RequestContentType:  "",
		Response:            new(preferencetypes.Preference),
		ResponseContentType: "application/json",
		SuccessStatusCode:   http.StatusOK,
		ErrorStatusCodes:    []int{http.StatusBadRequest, http.StatusNotFound},
		Deprecated:          false,
		SecuritySchemes:     newSecuritySchemes(types.RoleAdmin),
	}))

	router.Put("/v1/o11y/org/preferences/{name}", handler.New(provider.authzMiddleware.AdminAccess(provider.preferenceHandler.UpdateByOrg), handler.OpenAPIDef{
		ID:                  "UpdateOrgPreference",
		Tags:                []string{"preferences"},
		Summary:             "Update org preference",
		Description:         "This endpoint updates the org preference by name",
		Request:             new(preferencetypes.UpdatablePreference),
		RequestContentType:  "application/json",
		Response:            nil,
		ResponseContentType: "",
		SuccessStatusCode:   http.StatusNoContent,
		ErrorStatusCodes:    []int{http.StatusBadRequest, http.StatusNotFound},
		Deprecated:          false,
		SecuritySchemes:     newSecuritySchemes(types.RoleAdmin),
	}))
}
