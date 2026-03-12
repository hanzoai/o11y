package api

import (
	"net/http"

	baseapp "github.com/hanzoai/o11y/pkg/query-service/app"
	basemodel "github.com/hanzoai/o11y/pkg/query-service/model"
)

func RespondError(w http.ResponseWriter, apiErr basemodel.BaseApiError, data interface{}) {
	baseapp.RespondError(w, apiErr, data)
}
