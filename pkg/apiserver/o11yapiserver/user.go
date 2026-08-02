package o11yapiserver

import (
	"net/http"

	"github.com/hanzoai/o11y/pkg/http/handler"
	"github.com/hanzoai/o11y/pkg/http/routing"
	"github.com/hanzoai/o11y/pkg/types"
	"github.com/hanzoai/o11y/pkg/types/authtypes"
)

// ALL of these routes are ALSO declared as typed ops at the module's mount seam
// (identity.go in the repo root, minus none of this file), which is what carries
// them into the composed document, the SDK, the CLI and the agent surface. That
// is a second DISPATCH, never a second implementation: the ops answer by handing
// the call to this router, so the handlers below stay the one place the work is
// performed — and the gates declared here (AdminAccess, SelfAccess, OpenAccess)
// stay the one place access is decided. Both halves are needed — the ops serve
// the composed binary, this router serves the standalone process, which has no
// native router to register an op on — so deleting either drops one of the two
// deployments.
func (provider *provider) addUserRoutes(router routing.Router) {
	router.Post("/v1/o11y/invite", handler.New(provider.authzMiddleware.AdminAccess(provider.userHandler.CreateInvite), handler.OpenAPIDef{
		ID:                  "CreateInvite",
		Tags:                []string{"users"},
		Summary:             "Create invite",
		Description:         "This endpoint creates an invite for a user",
		Request:             new(types.PostableInvite),
		RequestContentType:  "application/json",
		Response:            new(types.Invite),
		ResponseContentType: "application/json",
		SuccessStatusCode:   http.StatusCreated,
		ErrorStatusCodes:    []int{http.StatusBadRequest, http.StatusConflict},
		Deprecated:          true,
		SecuritySchemes:     newSecuritySchemes(types.RoleAdmin),
	}))

	router.Post("/v1/o11y/invite/bulk", handler.New(provider.authzMiddleware.AdminAccess(provider.userHandler.CreateBulkInvite), handler.OpenAPIDef{
		ID:                 "CreateBulkInvite",
		Tags:               []string{"users"},
		Summary:            "Create bulk invite",
		Description:        "This endpoint creates a bulk invite for a user",
		Request:            new(types.PostableBulkInviteRequest),
		RequestContentType: "application/json",
		Response:           nil,
		SuccessStatusCode:  http.StatusCreated,
		ErrorStatusCodes:   []int{http.StatusBadRequest, http.StatusConflict},
		Deprecated:         true,
		SecuritySchemes:    newSecuritySchemes(types.RoleAdmin),
	}))

	router.Get("/v1/o11y/user", handler.New(provider.authzMiddleware.AdminAccess(provider.userHandler.ListUsersDeprecated), handler.OpenAPIDef{
		ID:                  "ListUsersDeprecated",
		Tags:                []string{"users"},
		Summary:             "List users",
		Description:         "This endpoint lists all users",
		Request:             nil,
		RequestContentType:  "",
		Response:            make([]*types.DeprecatedUser, 0),
		ResponseContentType: "application/json",
		SuccessStatusCode:   http.StatusOK,
		ErrorStatusCodes:    []int{},
		Deprecated:          true,
		SecuritySchemes:     newSecuritySchemes(types.RoleAdmin),
	}))

	router.Get("/v1/o11y/users", handler.New(provider.authzMiddleware.AdminAccess(provider.userHandler.ListUsers), handler.OpenAPIDef{
		ID:                  "ListUsers",
		Tags:                []string{"users"},
		Summary:             "List users v2",
		Description:         "This endpoint lists all users for the organization",
		Request:             nil,
		RequestContentType:  "",
		Response:            make([]*types.User, 0),
		ResponseContentType: "application/json",
		SuccessStatusCode:   http.StatusOK,
		ErrorStatusCodes:    []int{},
		Deprecated:          false,
		SecuritySchemes:     newSecuritySchemes(types.RoleAdmin),
	}))

	router.Get("/v1/o11y/user/me", handler.New(provider.authzMiddleware.OpenAccess(provider.userHandler.GetMyUserDeprecated), handler.OpenAPIDef{
		ID:                  "GetMyUserDeprecated",
		Tags:                []string{"users"},
		Summary:             "Get my user",
		Description:         "This endpoint returns the user I belong to",
		Request:             nil,
		RequestContentType:  "",
		Response:            new(types.DeprecatedUser),
		ResponseContentType: "application/json",
		SuccessStatusCode:   http.StatusOK,
		ErrorStatusCodes:    []int{},
		Deprecated:          true,
		SecuritySchemes:     []handler.OpenAPISecurityScheme{{Name: authtypes.IdentNProviderTokenizer.StringValue()}},
	}))

	router.Get("/v1/o11y/users/me", handler.New(provider.authzMiddleware.OpenAccess(provider.userHandler.GetMyUser), handler.OpenAPIDef{
		ID:                  "GetMyUser",
		Tags:                []string{"users"},
		Summary:             "Get my user v2",
		Description:         "This endpoint returns the user I belong to",
		Request:             nil,
		RequestContentType:  "",
		Response:            new(authtypes.UserWithRoles),
		ResponseContentType: "application/json",
		SuccessStatusCode:   http.StatusOK,
		ErrorStatusCodes:    []int{},
		Deprecated:          false,
		SecuritySchemes:     []handler.OpenAPISecurityScheme{{Name: authtypes.IdentNProviderTokenizer.StringValue()}},
	}))

	router.Post("/v1/o11y/users", handler.New(provider.authzMiddleware.AdminAccess(provider.userHandler.CreateUser), handler.OpenAPIDef{
		ID:                  "CreateUser",
		Tags:                []string{"users"},
		Summary:             "Create user",
		Description:         "This endpoint creates a user for the organization",
		Request:             new(authtypes.PostableUser),
		RequestContentType:  "application/json",
		Response:            new(types.Identifiable),
		ResponseContentType: "application/json",
		SuccessStatusCode:   http.StatusCreated,
		ErrorStatusCodes:    []int{http.StatusBadRequest, http.StatusConflict},
		Deprecated:          false,
		SecuritySchemes:     newSecuritySchemes(types.RoleAdmin),
	}))

	router.Put("/v1/o11y/users/me", handler.New(provider.authzMiddleware.OpenAccess(provider.userHandler.UpdateMyUser), handler.OpenAPIDef{
		ID:                  "UpdateMyUserV2",
		Tags:                []string{"users"},
		Summary:             "Update my user v2",
		Description:         "This endpoint updates the user I belong to",
		Request:             new(types.UpdatableUser),
		RequestContentType:  "application/json",
		Response:            nil,
		ResponseContentType: "",
		SuccessStatusCode:   http.StatusNoContent,
		ErrorStatusCodes:    []int{},
		Deprecated:          false,
		SecuritySchemes:     []handler.OpenAPISecurityScheme{{Name: authtypes.IdentNProviderTokenizer.StringValue()}},
	}))

	router.Get("/v1/o11y/user/{id}", handler.New(provider.authzMiddleware.SelfAccess(provider.userHandler.GetUserDeprecated), handler.OpenAPIDef{
		ID:                  "GetUserDeprecated",
		Tags:                []string{"users"},
		Summary:             "Get user",
		Description:         "This endpoint returns the user by id",
		Request:             nil,
		RequestContentType:  "",
		Response:            new(types.DeprecatedUser),
		ResponseContentType: "application/json",
		SuccessStatusCode:   http.StatusOK,
		ErrorStatusCodes:    []int{http.StatusNotFound},
		Deprecated:          true,
		SecuritySchemes:     newSecuritySchemes(types.RoleAdmin),
	}))

	router.Get("/v1/o11y/users/{id}", handler.New(provider.authzMiddleware.AdminAccess(provider.userHandler.GetUser), handler.OpenAPIDef{
		ID:                  "GetUser",
		Tags:                []string{"users"},
		Summary:             "Get user by user id",
		Description:         "This endpoint returns the user by id",
		Request:             nil,
		RequestContentType:  "",
		Response:            new(authtypes.UserWithRoles),
		ResponseContentType: "application/json",
		SuccessStatusCode:   http.StatusOK,
		ErrorStatusCodes:    []int{http.StatusNotFound},
		Deprecated:          false,
		SecuritySchemes:     newSecuritySchemes(types.RoleAdmin),
	}))

	router.Put("/v1/o11y/user/{id}", handler.New(provider.authzMiddleware.SelfAccess(provider.userHandler.UpdateUserDeprecated), handler.OpenAPIDef{
		ID:                  "UpdateUserDeprecated",
		Tags:                []string{"users"},
		Summary:             "Update user",
		Description:         "This endpoint updates the user by id",
		Request:             new(types.DeprecatedUser),
		RequestContentType:  "application/json",
		Response:            new(types.DeprecatedUser),
		ResponseContentType: "application/json",
		SuccessStatusCode:   http.StatusOK,
		ErrorStatusCodes:    []int{http.StatusBadRequest, http.StatusNotFound},
		Deprecated:          true,
		SecuritySchemes:     newSecuritySchemes(types.RoleAdmin),
	}))

	router.Put("/v1/o11y/users/{id}", handler.New(provider.authzMiddleware.AdminAccess(provider.userHandler.UpdateUser), handler.OpenAPIDef{
		ID:                  "UpdateUser",
		Tags:                []string{"users"},
		Summary:             "Update user v2",
		Description:         "This endpoint updates the user by id",
		Request:             new(types.UpdatableUser),
		RequestContentType:  "application/json",
		Response:            nil,
		ResponseContentType: "",
		SuccessStatusCode:   http.StatusNoContent,
		ErrorStatusCodes:    []int{http.StatusBadRequest, http.StatusNotFound},
		Deprecated:          false,
		SecuritySchemes:     newSecuritySchemes(types.RoleAdmin),
	}))

	router.Delete("/v1/o11y/user/{id}", handler.New(provider.authzMiddleware.AdminAccess(provider.userHandler.DeleteUser), handler.OpenAPIDef{
		ID:                  "DeleteUserDeprecated",
		Tags:                []string{"users"},
		Summary:             "Delete user",
		Description:         "This endpoint deletes the user by id",
		Request:             nil,
		RequestContentType:  "",
		Response:            nil,
		ResponseContentType: "",
		SuccessStatusCode:   http.StatusNoContent,
		ErrorStatusCodes:    []int{http.StatusNotFound},
		Deprecated:          true,
		SecuritySchemes:     newSecuritySchemes(types.RoleAdmin),
	}))

	router.Delete("/v1/o11y/users/{id}", handler.New(provider.authzMiddleware.AdminAccess(provider.userHandler.DeleteUser), handler.OpenAPIDef{
		ID:                  "DeleteUser",
		Tags:                []string{"users"},
		Summary:             "Delete user",
		Description:         "This endpoint deletes the user by id",
		Request:             nil,
		RequestContentType:  "",
		Response:            nil,
		ResponseContentType: "",
		SuccessStatusCode:   http.StatusNoContent,
		ErrorStatusCodes:    []int{http.StatusNotFound},
		Deprecated:          false,
		SecuritySchemes:     newSecuritySchemes(types.RoleAdmin),
	}))

	router.Get("/v1/o11y/getResetPasswordToken/{id}", handler.New(provider.authzMiddleware.AdminAccess(provider.userHandler.GetResetPasswordTokenDeprecated), handler.OpenAPIDef{
		ID:                  "GetResetPasswordTokenDeprecated",
		Tags:                []string{"users"},
		Summary:             "Get reset password token",
		Description:         "This endpoint returns the reset password token by id",
		Request:             nil,
		RequestContentType:  "",
		Response:            new(types.ResetPasswordToken),
		ResponseContentType: "application/json",
		SuccessStatusCode:   http.StatusOK,
		ErrorStatusCodes:    []int{http.StatusBadRequest, http.StatusNotFound},
		Deprecated:          true,
		SecuritySchemes:     newSecuritySchemes(types.RoleAdmin),
	}))

	router.Get("/v1/o11y/users/{id}/reset_password_tokens", handler.New(provider.authzMiddleware.AdminAccess(provider.userHandler.GetResetPasswordToken), handler.OpenAPIDef{
		ID:                  "GetResetPasswordToken",
		Tags:                []string{"users"},
		Summary:             "Get reset password token for a user",
		Description:         "This endpoint returns the existing reset password token for a user.",
		Request:             nil,
		RequestContentType:  "",
		Response:            new(types.ResetPasswordToken),
		ResponseContentType: "application/json",
		SuccessStatusCode:   http.StatusOK,
		ErrorStatusCodes:    []int{http.StatusNotFound},
		Deprecated:          false,
		SecuritySchemes:     newSecuritySchemes(types.RoleAdmin),
	}))

	router.Put("/v1/o11y/users/{id}/reset_password_tokens", handler.New(provider.authzMiddleware.AdminAccess(provider.userHandler.CreateResetPasswordToken), handler.OpenAPIDef{
		ID:                  "CreateResetPasswordToken",
		Tags:                []string{"users"},
		Summary:             "Create or regenerate reset password token for a user",
		Description:         "This endpoint creates or regenerates a reset password token for a user. If a valid token exists, it is returned. If expired, a new one is created.",
		Request:             nil,
		RequestContentType:  "",
		Response:            new(types.ResetPasswordToken),
		ResponseContentType: "application/json",
		SuccessStatusCode:   http.StatusCreated,
		ErrorStatusCodes:    []int{http.StatusBadRequest, http.StatusNotFound},
		Deprecated:          false,
		SecuritySchemes:     newSecuritySchemes(types.RoleAdmin),
	}))

	router.Post("/v1/o11y/reset_password_tokens/verify", handler.New(provider.authzMiddleware.OpenAccess(provider.userHandler.VerifyResetPasswordToken), handler.OpenAPIDef{
		ID:                  "VerifyResetPasswordToken",
		Tags:                []string{"users"},
		Summary:             "Verify a reset password token",
		Description:         "This endpoint verifies whether a reset password token exists and is not expired",
		Request:             new(types.PostableVerifyResetPasswordToken),
		RequestContentType:  "application/json",
		Response:            nil,
		ResponseContentType: "",
		SuccessStatusCode:   http.StatusNoContent,
		ErrorStatusCodes:    []int{http.StatusBadRequest, http.StatusNotFound},
		Deprecated:          false,
		SecuritySchemes:     []handler.OpenAPISecurityScheme{},
	}))

	router.Post("/v1/o11y/resetPassword", handler.New(provider.authzMiddleware.OpenAccess(provider.userHandler.ResetPassword), handler.OpenAPIDef{
		ID:                  "ResetPassword",
		Tags:                []string{"users"},
		Summary:             "Reset password",
		Description:         "This endpoint resets the password by token",
		Request:             new(types.PostableResetPassword),
		RequestContentType:  "application/json",
		Response:            nil,
		ResponseContentType: "",
		SuccessStatusCode:   http.StatusNoContent,
		ErrorStatusCodes:    []int{http.StatusBadRequest, http.StatusConflict},
		Deprecated:          false,
		SecuritySchemes:     []handler.OpenAPISecurityScheme{},
	}))

	router.Put("/v1/o11y/users/me/factor_password", handler.New(provider.authzMiddleware.OpenAccess(provider.userHandler.ChangePassword), handler.OpenAPIDef{
		ID:                  "UpdateMyPassword",
		Tags:                []string{"users"},
		Summary:             "Updates my password",
		Description:         "This endpoint updates the password of the user I belong to",
		Request:             new(types.ChangePasswordRequest),
		RequestContentType:  "application/json",
		Response:            nil,
		ResponseContentType: "",
		SuccessStatusCode:   http.StatusNoContent,
		ErrorStatusCodes:    []int{http.StatusBadRequest, http.StatusNotFound},
		Deprecated:          false,
		SecuritySchemes:     newSecuritySchemes(types.RoleAdmin),
	}))

	router.Post("/v1/o11y/factor_password/forgot", handler.New(provider.authzMiddleware.OpenAccess(provider.userHandler.ForgotPassword), handler.OpenAPIDef{
		ID:                  "ForgotPassword",
		Tags:                []string{"users"},
		Summary:             "Forgot password",
		Description:         "This endpoint initiates the forgot password flow by sending a reset password email",
		Request:             new(types.PostableForgotPassword),
		RequestContentType:  "application/json",
		Response:            nil,
		ResponseContentType: "",
		SuccessStatusCode:   http.StatusNoContent,
		ErrorStatusCodes:    []int{http.StatusBadRequest, http.StatusUnprocessableEntity},
		Deprecated:          false,
		SecuritySchemes:     []handler.OpenAPISecurityScheme{},
	}))

	router.Get("/v1/o11y/users/{id}/roles", handler.New(provider.authzMiddleware.AdminAccess(provider.userHandler.GetRolesByUserID), handler.OpenAPIDef{
		ID:                  "GetRolesByUserID",
		Tags:                []string{"users"},
		Summary:             "Get user roles",
		Description:         "This endpoint returns the user roles by user id",
		Request:             nil,
		RequestContentType:  "",
		Response:            make([]*authtypes.Role, 0),
		ResponseContentType: "application/json",
		SuccessStatusCode:   http.StatusOK,
		ErrorStatusCodes:    []int{http.StatusNotFound},
		Deprecated:          false,
		SecuritySchemes:     newSecuritySchemes(types.RoleAdmin),
	}))

	router.Post("/v1/o11y/users/{id}/roles", handler.New(provider.authzMiddleware.AdminAccess(provider.userHandler.SetRoleByUserID), handler.OpenAPIDef{
		ID:                  "SetRoleByUserID",
		Tags:                []string{"users"},
		Summary:             "Set user roles",
		Description:         "This endpoint assigns the role to the user roles by user id",
		Request:             new(types.PostableRole),
		RequestContentType:  "application/json",
		Response:            nil,
		ResponseContentType: "",
		SuccessStatusCode:   http.StatusOK,
		ErrorStatusCodes:    []int{http.StatusNotFound},
		Deprecated:          false,
		SecuritySchemes:     newSecuritySchemes(types.RoleAdmin),
	}))

	router.Delete("/v1/o11y/users/{id}/roles/{roleId}", handler.New(provider.authzMiddleware.AdminAccess(provider.userHandler.RemoveUserRoleByRoleID), handler.OpenAPIDef{
		ID:                  "RemoveUserRoleByUserIDAndRoleID",
		Tags:                []string{"users"},
		Summary:             "Remove a role from user",
		Description:         "This endpoint removes a role from the user by user id and role id",
		Request:             nil,
		RequestContentType:  "",
		Response:            nil,
		ResponseContentType: "",
		SuccessStatusCode:   http.StatusNoContent,
		ErrorStatusCodes:    []int{http.StatusNotFound},
		Deprecated:          false,
		SecuritySchemes:     newSecuritySchemes(types.RoleAdmin),
	}))

	router.Get("/v1/o11y/roles/{id}/users", handler.New(provider.authzMiddleware.AdminAccess(provider.userHandler.GetUsersByRoleID), handler.OpenAPIDef{
		ID:                  "GetUsersByRoleID",
		Tags:                []string{"users"},
		Summary:             "Get users by role id",
		Description:         "This endpoint returns the users having the role by role id",
		Request:             nil,
		RequestContentType:  "",
		Response:            make([]*types.User, 0),
		ResponseContentType: "application/json",
		SuccessStatusCode:   http.StatusOK,
		ErrorStatusCodes:    []int{http.StatusNotFound},
		Deprecated:          false,
		SecuritySchemes:     newSecuritySchemes(types.RoleAdmin),
	}))
}
