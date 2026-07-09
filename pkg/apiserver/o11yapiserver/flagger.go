package o11yapiserver

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/hanzoai/o11y/pkg/http/handler"
	"github.com/hanzoai/o11y/pkg/types"
	"github.com/hanzoai/o11y/pkg/types/featuretypes"
)

func (provider *provider) addFlaggerRoutes(router *mux.Router) error {
	if err := router.Handle("/api/v2/features", handler.New(provider.authzMiddleware.ViewAccess(provider.flaggerHandler.GetFeatures), handler.OpenAPIDef{
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
	})).Methods(http.MethodGet).GetError(); err != nil {
		return err
	}

	return nil
}
