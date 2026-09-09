package o11yapiserver

import (
	"net/http"

	"github.com/hanzoai/o11y/pkg/http/handler"
	"github.com/hanzoai/o11y/pkg/http/routing"
	"github.com/hanzoai/o11y/pkg/types/authtypes"
)

// ONE user route survives, and it is a READ of the caller's OWN identity.
//
// o11y used to answer twenty-five routes here: invites, passwords, reset
// tokens, member listing, member editing, role assignment. Every one was a
// SECOND ANSWER to a question Hanzo IAM already answers, and a second answer is
// a second place to get it wrong — a password store o11y must hash and rotate,
// an invite link o11y must expire, a member edit that disagrees with the one
// IAM holds. They are DELETED, with their handlers, their stores and their
// tables, not disabled: a surface that is only turned off is still a surface
// someone turns back on.
//
// GetMyUser stays because the console cannot render without knowing WHO it is
// rendering for, and this is the one identity question o11y can answer from
// what the edge already told it. It reads the CLAIMS, not a row (see
// impluser.GetMyUser). That is the whole of the difference: o11y REPORTS the
// identity it was handed; it does not keep one.
//
// OpenAccess, unchanged: the gate on this route has always been "is anyone
// authenticated", never a role, because the answer is about the caller alone.
//
// It is also declared as a typed op at the module's mount seam (identity.go in
// the repo root), which carries it into the composed document, the SDK, the CLI
// and the agent surface. That is a second DISPATCH, never a second
// implementation: the op answers by handing the call to this router.
func (provider *provider) addUserRoutes(router routing.Router) {
	router.Get("/v1/o11y/users/me", handler.New(provider.authzMiddleware.OpenAccess(provider.userHandler.GetMyUser), handler.OpenAPIDef{
		ID:                  "GetMyUser",
		Tags:                []string{"users"},
		Summary:             "Get my user",
		Description:         "This endpoint returns the identity the edge asserted for the caller",
		Request:             nil,
		RequestContentType:  "",
		Response:            new(authtypes.UserWithRoles),
		ResponseContentType: "application/json",
		SuccessStatusCode:   http.StatusOK,
		ErrorStatusCodes:    []int{},
		Deprecated:          false,
		SecuritySchemes:     []handler.OpenAPISecurityScheme{{Name: authtypes.IdentNProviderIAM.StringValue()}},
	}))
}
