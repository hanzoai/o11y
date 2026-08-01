package o11yapiserver

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/hanzoai/o11y/pkg/http/handler"
	"github.com/hanzoai/o11y/pkg/types"
	"github.com/hanzoai/o11y/pkg/types/zeustypes"
)

// ALL of these routes are ALSO declared as typed ops at the module's mount seam
// (integrations.go in the repo root), which is what carries them into the
// composed document, the SDK, the CLI and the agent surface. That is a second
// DISPATCH, never a second implementation: the ops answer by handing the call
// to this router, so the handlers below stay the one place the work is
// performed — and the gates declared here (AdminAccess, ViewAccess) stay the
// one place access is decided. Both halves are needed — the ops serve the
// composed binary, this router serves the standalone process, which has no
// native router to register an op on — so deleting either drops one of the two
// deployments.
func (provider *provider) addZeusRoutes(router *mux.Router) error {
	if err := router.Handle("/v1/o11y/zeus/profiles", handler.New(provider.authzMiddleware.AdminAccess(provider.zeusHandler.PutProfile), handler.OpenAPIDef{
		ID:                  "PutProfile",
		Tags:                []string{"zeus"},
		Summary:             "Put profile in Zeus for a deployment.",
		Description:         "This endpoint saves the profile of a deployment to zeus.",
		Request:             new(zeustypes.PostableProfile),
		RequestContentType:  "application/json",
		Response:            nil,
		ResponseContentType: "",
		SuccessStatusCode:   http.StatusNoContent,
		ErrorStatusCodes:    []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusConflict},
		Deprecated:          false,
		SecuritySchemes:     newSecuritySchemes(types.RoleAdmin),
	})).Methods(http.MethodPut).GetError(); err != nil {
		return err
	}

	if err := router.Handle("/v1/o11y/zeus/hosts", handler.New(provider.authzMiddleware.ViewAccess(provider.zeusHandler.GetHosts), handler.OpenAPIDef{
		ID:                  "GetHosts",
		Tags:                []string{"zeus"},
		Summary:             "Get host info from Zeus.",
		Description:         "This endpoint gets the host info from zeus.",
		Request:             nil,
		RequestContentType:  "",
		Response:            new(zeustypes.GettableHost),
		ResponseContentType: "application/json",
		SuccessStatusCode:   http.StatusOK,
		ErrorStatusCodes:    []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound},
		Deprecated:          false,
		SecuritySchemes:     newSecuritySchemes(types.RoleViewer),
	})).Methods(http.MethodGet).GetError(); err != nil {
		return err
	}

	if err := router.Handle("/v1/o11y/zeus/hosts", handler.New(provider.authzMiddleware.AdminAccess(provider.zeusHandler.PutHost), handler.OpenAPIDef{
		ID:                  "PutHost",
		Tags:                []string{"zeus"},
		Summary:             "Put host in Zeus for a deployment.",
		Description:         "This endpoint saves the host of a deployment to zeus.",
		Request:             new(zeustypes.PostableHost),
		RequestContentType:  "application/json",
		Response:            nil,
		ResponseContentType: "",
		SuccessStatusCode:   http.StatusNoContent,
		ErrorStatusCodes:    []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusConflict},
		Deprecated:          false,
		SecuritySchemes:     newSecuritySchemes(types.RoleAdmin),
	})).Methods(http.MethodPut).GetError(); err != nil {
		return err
	}

	return nil
}
