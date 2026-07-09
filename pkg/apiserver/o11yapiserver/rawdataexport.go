package o11yapiserver

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/hanzoai/o11y/pkg/http/handler"
	"github.com/hanzoai/o11y/pkg/types"
	"github.com/hanzoai/o11y/pkg/types/exporttypes"
	v5 "github.com/hanzoai/o11y/pkg/types/querybuildertypes/querybuildertypesv5"
)

func (provider *provider) addRawDataExportRoutes(router *mux.Router) error {

	if err := router.Handle("/api/v1/export_raw_data", handler.New(provider.authzMiddleware.ViewAccess(provider.rawDataExportHandler.ExportRawData), handler.OpenAPIDef{
		ID:                  "HandleExportRawDataPOST",
		Tags:                []string{"logs", "traces"},
		Summary:             "Export raw data",
		Description:         "This endpoints allows complex query exporting raw data for traces and logs",
		Request:             new(v5.QueryRangeRequest),
		RequestQuery:        new(exporttypes.ExportRawDataFormatQueryParam),
		RequestContentType:  "application/json",
		Response:            nil,
		ResponseContentType: "application/json",
		SuccessStatusCode:   http.StatusOK,
		ErrorStatusCodes:    []int{http.StatusBadRequest},
		SecuritySchemes:     newSecuritySchemes(types.RoleViewer),
	})).Methods(http.MethodPost).GetError(); err != nil {
		return err
	}

	return nil
}
