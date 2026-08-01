package o11yapiserver

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/hanzoai/o11y/pkg/http/handler"
	"github.com/hanzoai/o11y/pkg/types"
	"github.com/hanzoai/o11y/pkg/types/authtypes"
	"github.com/hanzoai/o11y/pkg/types/coretypes"
)

// ALL FIVE role routes are ALSO declared as typed ops at the module's mount
// seam (access.go in the repo root), which is what carries them into the
// composed document, the SDK, the CLI and the agent surface. That is a second
// DISPATCH, never a second implementation: the ops answer by handing the call
// to this router, so the CheckResources gate and the handlers below stay the
// one place role access control is performed. Both are needed — the ops serve
// the composed binary, this router serves the standalone process, which has no
// native router to register an op on — so deleting either half drops one of
// the two deployments.
func (provider *provider) addRoleRoutes(router *mux.Router) error {
	if err := router.Handle("/v1/o11y/roles", handler.New(
		provider.authzMiddleware.CheckResources(provider.authzHandler.Create, authtypes.O11yAdminRoleName),
		handler.OpenAPIDef{
			ID:                  "CreateRole",
			Tags:                []string{"role"},
			Summary:             "Create role",
			Description:         "This endpoint creates a role",
			Request:             new(authtypes.PostableRole),
			RequestContentType:  "",
			Response:            new(types.Identifiable),
			ResponseContentType: "application/json",
			SuccessStatusCode:   http.StatusCreated,
			ErrorStatusCodes:    []int{http.StatusBadRequest, http.StatusConflict, http.StatusNotImplemented, http.StatusUnavailableForLegalReasons},
			Deprecated:          false,
			SecuritySchemes:     newScopedSecuritySchemes([]string{coretypes.ResourceRole.Scope(coretypes.VerbCreate)}),
		},
		handler.WithResourceDefs(handler.BasicResourceDef{
			Resource: coretypes.ResourceRole,
			Verb:     coretypes.VerbCreate,
			Category: coretypes.ActionCategoryAccessControl,
			ID:       coretypes.ResponseJSONPath("data.id"),
			Selector: coretypes.WildcardSelector,
		}),
	)).Methods(http.MethodPost).GetError(); err != nil {
		return err
	}

	if err := router.Handle("/v1/o11y/roles", handler.New(
		provider.authzMiddleware.CheckResources(provider.authzHandler.List, authtypes.O11yAdminRoleName),
		handler.OpenAPIDef{
			ID:                  "ListRoles",
			Tags:                []string{"role"},
			Summary:             "List roles",
			Description:         "This endpoint lists all roles",
			Request:             nil,
			RequestContentType:  "",
			Response:            make([]*authtypes.Role, 0),
			ResponseContentType: "application/json",
			SuccessStatusCode:   http.StatusOK,
			ErrorStatusCodes:    []int{},
			Deprecated:          false,
			SecuritySchemes:     newScopedSecuritySchemes([]string{coretypes.ResourceRole.Scope(coretypes.VerbList)}),
		},
		handler.WithResourceDefs(handler.BasicResourceDef{
			Resource: coretypes.ResourceRole,
			Verb:     coretypes.VerbList,
			Category: coretypes.ActionCategoryAccessControl,
			Selector: coretypes.WildcardSelector,
		}),
	)).Methods(http.MethodGet).GetError(); err != nil {
		return err
	}

	if err := router.Handle("/v1/o11y/roles/{id}", handler.New(
		provider.authzMiddleware.CheckResources(provider.authzHandler.Get, authtypes.O11yAdminRoleName),
		handler.OpenAPIDef{
			ID:                  "GetRole",
			Tags:                []string{"role"},
			Summary:             "Get role",
			Description:         "This endpoint gets a role",
			Request:             nil,
			RequestContentType:  "",
			Response:            new(authtypes.RoleWithTransactionGroups),
			ResponseContentType: "application/json",
			SuccessStatusCode:   http.StatusOK,
			ErrorStatusCodes:    []int{},
			Deprecated:          false,
			SecuritySchemes:     newScopedSecuritySchemes([]string{coretypes.ResourceRole.Scope(coretypes.VerbRead)}),
		},
		handler.WithResourceDefs(handler.BasicResourceDef{
			Resource: coretypes.ResourceRole,
			Verb:     coretypes.VerbRead,
			Category: coretypes.ActionCategoryAccessControl,
			ID:       coretypes.PathParam("id"),
			Selector: provider.roleSelector,
		}),
	)).Methods(http.MethodGet).GetError(); err != nil {
		return err
	}

	if err := router.Handle("/v1/o11y/roles/{id}", handler.New(
		provider.authzMiddleware.CheckResources(provider.authzHandler.Update, authtypes.O11yAdminRoleName),
		handler.OpenAPIDef{
			ID:                  "UpdateRole",
			Tags:                []string{"role"},
			Summary:             "Update role",
			Description:         "This endpoint updates a role",
			Request:             new(authtypes.UpdatableRole),
			RequestContentType:  "",
			Response:            nil,
			ResponseContentType: "application/json",
			SuccessStatusCode:   http.StatusNoContent,
			ErrorStatusCodes:    []int{http.StatusNotFound, http.StatusNotImplemented, http.StatusUnavailableForLegalReasons},
			Deprecated:          false,
			SecuritySchemes:     newScopedSecuritySchemes([]string{coretypes.ResourceRole.Scope(coretypes.VerbUpdate)}),
		},
		handler.WithResourceDefs(handler.BasicResourceDef{
			Resource: coretypes.ResourceRole,
			Verb:     coretypes.VerbUpdate,
			Category: coretypes.ActionCategoryAccessControl,
			ID:       coretypes.PathParam("id"),
			Selector: provider.roleSelector,
		}),
	)).Methods(http.MethodPut).GetError(); err != nil {
		return err
	}

	if err := router.Handle("/v1/o11y/roles/{id}", handler.New(
		provider.authzMiddleware.CheckResources(provider.authzHandler.Delete, authtypes.O11yAdminRoleName),
		handler.OpenAPIDef{
			ID:                  "DeleteRole",
			Tags:                []string{"role"},
			Summary:             "Delete role",
			Description:         "This endpoint deletes a role",
			Request:             nil,
			RequestContentType:  "",
			Response:            nil,
			ResponseContentType: "application/json",
			SuccessStatusCode:   http.StatusNoContent,
			ErrorStatusCodes:    []int{http.StatusNotFound, http.StatusNotImplemented, http.StatusUnavailableForLegalReasons},
			Deprecated:          false,
			SecuritySchemes:     newScopedSecuritySchemes([]string{coretypes.ResourceRole.Scope(coretypes.VerbDelete)}),
		},
		handler.WithResourceDefs(handler.BasicResourceDef{
			Resource: coretypes.ResourceRole,
			Verb:     coretypes.VerbDelete,
			Category: coretypes.ActionCategoryAccessControl,
			ID:       coretypes.PathParam("id"),
			Selector: provider.roleSelector,
		}),
	)).Methods(http.MethodDelete).GetError(); err != nil {
		return err
	}

	return nil
}
