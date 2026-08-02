package o11yapiserver

import (
	"net/http"

	"github.com/hanzoai/o11y/pkg/http/handler"
	"github.com/hanzoai/o11y/pkg/http/routing"
	"github.com/hanzoai/o11y/pkg/types"
	"github.com/hanzoai/o11y/pkg/types/promotetypes"
)

// addPromoteRoutes registers the two promotion routes on the runtime's own
// router.
//
// TWO DISPATCHES, ONE IMPLEMENTATION. These registrations stay: the standalone
// server has no native router to register an op on. The composed binary
// reaches the SAME handlers through the typed ops in the repo root's logs.go
// (logPromote, logPromoted), which relay here — deleting either half drops one
// of the two deployments.
func (provider *provider) addPromoteRoutes(router routing.Router) {
	router.Post("/v1/o11y/logs/promote_paths", handler.New(provider.authzMiddleware.EditAccess(provider.promoteHandler.HandlePromoteAndIndexPaths), handler.OpenAPIDef{
		ID:                  "HandlePromoteAndIndexPaths",
		Tags:                []string{"logs"},
		Summary:             "Promote and index paths",
		Description:         "This endpoints promotes and indexes paths",
		Request:             new([]*promotetypes.PromotePath),
		RequestContentType:  "application/json",
		Response:            nil,
		ResponseContentType: "",
		SuccessStatusCode:   http.StatusCreated,
		ErrorStatusCodes:    []int{http.StatusBadRequest},
		SecuritySchemes:     newSecuritySchemes(types.RoleEditor),
	}))

	router.Get("/v1/o11y/logs/promote_paths", handler.New(provider.authzMiddleware.ViewAccess(provider.promoteHandler.ListPromotedAndIndexedPaths), handler.OpenAPIDef{
		ID:                  "ListPromotedAndIndexedPaths",
		Tags:                []string{"logs"},
		Summary:             "Promote and index paths",
		Description:         "This endpoints promotes and indexes paths",
		Request:             nil,
		RequestContentType:  "",
		Response:            new([]*promotetypes.PromotePath),
		ResponseContentType: "",
		SuccessStatusCode:   http.StatusOK,
		ErrorStatusCodes:    []int{http.StatusBadRequest},
		SecuritySchemes:     newSecuritySchemes(types.RoleViewer),
	}))
}
