package o11yapiserver

import (
	"net/http"

	"github.com/hanzoai/o11y/pkg/http/handler"
	"github.com/hanzoai/o11y/pkg/http/routing"
	"github.com/hanzoai/o11y/pkg/types"
)

// BOTH routes are ALSO declared as typed ops at the module's mount seam
// (identity.go in the repo root) — a second DISPATCH onto this router, never a
// second implementation; the AdminAccess gate below stays the one place access
// is decided. The ops serve the composed binary, this router the standalone
// process.
func (provider *provider) addOrgRoutes(router routing.Router) {
	router.Get("/v1/o11y/orgs/me", handler.New(provider.authzMiddleware.AdminAccess(provider.orgHandler.Get), handler.OpenAPIDef{
		ID:                  "GetMyOrganization",
		Tags:                []string{"orgs"},
		Summary:             "Get my organization",
		Description:         "This endpoint returns the organization I belong to",
		Request:             nil,
		RequestContentType:  "",
		Response:            new(types.Organization),
		ResponseContentType: "application/json",
		SuccessStatusCode:   http.StatusOK,
		ErrorStatusCodes:    []int{},
		Deprecated:          false,
		SecuritySchemes:     newSecuritySchemes(types.RoleAdmin),
	}))

	router.Put("/v1/o11y/orgs/me", handler.New(provider.authzMiddleware.AdminAccess(provider.orgHandler.Update), handler.OpenAPIDef{
		ID:                  "UpdateMyOrganization",
		Tags:                []string{"orgs"},
		Summary:             "Update my organization",
		Description:         "This endpoint updates the organization I belong to",
		Request:             new(types.Organization),
		RequestContentType:  "application/json",
		Response:            nil,
		ResponseContentType: "",
		SuccessStatusCode:   http.StatusNoContent,
		ErrorStatusCodes:    []int{http.StatusConflict, http.StatusBadRequest},
		Deprecated:          false,
		SecuritySchemes:     newSecuritySchemes(types.RoleAdmin),
	}))
}
