package o11yapiserver

import (
	"net/http"

	"github.com/hanzoai/o11y/pkg/http/handler"
	"github.com/hanzoai/o11y/pkg/http/routing"
	"github.com/hanzoai/o11y/pkg/types/globaltypes"
)

// addGlobalRoutes registers the one global-config route on the runtime's own
// router.
//
// TWO DISPATCHES, ONE IMPLEMENTATION. This registration stays: the standalone
// server has no native router to register an op on. The composed binary reaches
// the SAME handler through the typed op in the repo root's platform.go
// (globalConfig), which relays here — so the OpenAccess gate named below stays
// the one place access is decided, and deleting either half drops one of the
// two deployments.
func (provider *provider) addGlobalRoutes(router routing.Router) {
	router.Get("/v1/o11y/global/config", handler.New(provider.authzMiddleware.OpenAccess(provider.globalHandler.GetConfig), handler.OpenAPIDef{
		ID:                  "GetGlobalConfig",
		Tags:                []string{"global"},
		Summary:             "Get global config",
		Description:         "This endpoint returns global config",
		Request:             nil,
		RequestContentType:  "",
		Response:            new(globaltypes.Config),
		ResponseContentType: "application/json",
		SuccessStatusCode:   http.StatusOK,
		ErrorStatusCodes:    []int{},
		Deprecated:          false,
		SecuritySchemes:     nil,
	}))
}
