package o11yglobal

import (
	"net/http"

	"github.com/hanzoai/o11y/pkg/global"
	"github.com/hanzoai/o11y/pkg/http/render"
	"github.com/hanzoai/o11y/pkg/types"
)

type handler struct {
	global global.Global
}

func NewHandler(global global.Global) global.Handler {
	return &handler{global: global}
}

func (handler *handler) GetConfig(rw http.ResponseWriter, r *http.Request) {
	cfg := handler.global.GetConfig()

	render.Success(rw, http.StatusOK, types.NewGettableGlobalConfig(cfg.ExternalURL, cfg.IngestionURL))
}
