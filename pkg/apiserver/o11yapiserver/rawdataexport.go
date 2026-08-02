package o11yapiserver

import (
	"net/http"

	"github.com/hanzoai/o11y/pkg/http/handler"
	"github.com/hanzoai/o11y/pkg/http/routing"
	"github.com/hanzoai/o11y/pkg/types"
	"github.com/hanzoai/o11y/pkg/types/exporttypes"
	v5 "github.com/hanzoai/o11y/pkg/types/querybuildertypes/querybuildertypesv5"
)

// ESCAPE HATCH. export_raw_data is deliberately NOT a typed query-core op: it
// streams a chunked CSV/JSONL download with a Content-Disposition attachment and
// an X-Response-Complete trailer, so it has no JSON answer to name and the relay
// (which buffers a whole answer through an httptest recorder) would defeat the
// stream. It stays on the /v1/o11y delegation wildcard, byte-identical; see the
// escape-hatch record in querycore.go.
func (provider *provider) addRawDataExportRoutes(router routing.Router) {

	router.Post("/v1/o11y/export_raw_data", handler.New(provider.authzMiddleware.ViewAccess(provider.rawDataExportHandler.ExportRawData), handler.OpenAPIDef{
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
	}))
}
