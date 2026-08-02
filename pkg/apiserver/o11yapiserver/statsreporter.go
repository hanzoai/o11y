package o11yapiserver

import (
	"net/http"

	"github.com/hanzoai/o11y/pkg/http/handler"
	"github.com/hanzoai/o11y/pkg/http/routing"
	"github.com/hanzoai/o11y/pkg/types"
)

// addStatsReporterRoutes registers the one org-stats route on the runtime's own
// router.
//
// TWO DISPATCHES, ONE IMPLEMENTATION. This registration stays: the standalone
// server has no native router to register an op on. The composed binary reaches
// the SAME handler through the typed op in the repo root's platform.go
// (orgStats), which relays here — so the ViewAccess gate named below stays the
// one place access is decided, and deleting either half drops one of the two
// deployments.
func (provider *provider) addStatsReporterRoutes(router routing.Router) {
	router.Get("/v1/o11y/stats", handler.New(
		provider.authzMiddleware.ViewAccess(provider.statsHandler.Get),
		handler.OpenAPIDef{
			ID:                  "GetStats",
			Tags:                []string{"stats"},
			Summary:             "Get stats",
			Description:         "This endpoint returns the collected stats for the organization",
			Request:             nil,
			RequestContentType:  "",
			Response:            map[string]any{},
			ResponseContentType: "application/json",
			SuccessStatusCode:   http.StatusOK,
			ErrorStatusCodes:    []int{},
			Deprecated:          false,
			SecuritySchemes:     newSecuritySchemes(types.RoleViewer),
		},
	))
}
