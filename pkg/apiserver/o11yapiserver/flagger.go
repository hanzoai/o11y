package o11yapiserver

import (
	"net/http"

	"github.com/hanzoai/o11y/pkg/http/handler"
	"github.com/hanzoai/o11y/pkg/http/routing"
	"github.com/hanzoai/o11y/pkg/types"
	"github.com/hanzoai/o11y/pkg/types/featuretypes"
)

// addFlaggerRoutes registers the one features route on the runtime's own
// router.
//
// TWO DISPATCHES, ONE IMPLEMENTATION. This registration stays: the standalone
// server has no native router to register an op on. The composed binary reaches
// the SAME handler through the typed op in the repo root's platform.go
// (features), which relays here — so the ViewAccess gate named below stays the
// one place access is decided, and deleting either half drops one of the two
// deployments.
func (provider *provider) addFlaggerRoutes(router routing.Router) {
	router.Get("/v1/o11y/features", handler.New(provider.authzMiddleware.ViewAccess(provider.flaggerHandler.GetFeatures), handler.OpenAPIDef{
		ID:                  "GetFeatures",
		Tags:                []string{"features"},
		Summary:             "Get features",
		Description:         "This endpoint returns the supported features and their details",
		Request:             nil,
		RequestContentType:  "",
		Response:            make([]*featuretypes.GettableFeature, 0),
		ResponseContentType: "application/json",
		SuccessStatusCode:   http.StatusOK,
		ErrorStatusCodes:    []int{},
		Deprecated:          false,
		SecuritySchemes:     newSecuritySchemes(types.RoleViewer),
	}))
}
