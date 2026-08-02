package o11yapiserver

import (
	"net/http"

	"github.com/hanzoai/o11y/pkg/http/handler"
	"github.com/hanzoai/o11y/pkg/http/routing"
	"github.com/hanzoai/o11y/pkg/types/authtypes"
)

// The authz probe is ALSO declared as a typed op at the module's mount seam
// (access.go in the repo root), which is what carries it into the composed
// document, the SDK, the CLI and the agent surface. That is a second DISPATCH,
// never a second implementation: the op answers by handing the call to this
// router, so the handler below stays the one place the check is performed.
// Both are needed — the op serves the composed binary, this router serves the
// standalone process, which has no native router to register an op on.
func (provider *provider) addAuthzRoutes(router routing.Router) {
	router.Post("/v1/o11y/authz/check", handler.New(provider.authzHandler.Check, handler.OpenAPIDef{
		ID:                  "AuthzCheck",
		Tags:                []string{"authz"},
		Summary:             "Check permissions",
		Description:         "Checks if the authenticated user has permissions for given transactions",
		Request:             make([]*authtypes.Transaction, 0),
		RequestContentType:  "",
		Response:            make([]*authtypes.GettableTransaction, 0),
		ResponseContentType: "application/json",
		SuccessStatusCode:   http.StatusOK,
		ErrorStatusCodes:    []int{},
		Deprecated:          false,
		SecuritySchemes:     nil,
	}))
}
