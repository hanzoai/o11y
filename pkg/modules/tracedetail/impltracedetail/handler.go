package impltracedetail

import (
	"net/http"

	"github.com/hanzoai/o11y/pkg/http/binding"
	"github.com/hanzoai/o11y/pkg/http/render"
	"github.com/hanzoai/o11y/pkg/modules/tracedetail"
	"github.com/hanzoai/o11y/pkg/types/coretypes"
	"github.com/hanzoai/o11y/pkg/types/spantypes"
)

type handler struct {
	module tracedetail.Module
}

func NewHandler(module tracedetail.Module) tracedetail.Handler {
	return &handler{module: module}
}

func (h *handler) GetWaterfallV4(rw http.ResponseWriter, r *http.Request) {
	req := new(spantypes.PostableWaterfall)
	if err := binding.JSON.BindBody(r.Body, req); err != nil {
		render.Error(rw, err)
		return
	}

	if err := req.Validate(); err != nil {
		render.Error(rw, err)
		return
	}

	result, err := h.module.GetWaterfallV4(r.Context(), coretypes.Param(r, "traceId"), req.SelectedSpanID, req.UncollapsedSpans)
	if err != nil {
		render.Error(rw, err)
		return
	}

	render.Success(rw, http.StatusOK, result)
}

func (h *handler) GetTraceAggregations(rw http.ResponseWriter, r *http.Request) {
	req := new(spantypes.PostableTraceAggregations)
	if err := binding.JSON.BindBody(r.Body, req); err != nil {
		render.Error(rw, err)
		return
	}

	if err := req.Validate(); err != nil {
		render.Error(rw, err)
		return
	}

	result, err := h.module.GetTraceAggregations(r.Context(), coretypes.Param(r, "traceId"), req)
	if err != nil {
		render.Error(rw, err)
		return
	}

	render.Success(rw, http.StatusOK, result)
}

func (h *handler) GetFlamegraph(rw http.ResponseWriter, r *http.Request) {
	req := new(spantypes.PostableFlamegraph)
	if err := binding.JSON.BindBody(r.Body, req); err != nil {
		render.Error(rw, err)
		return
	}

	result, err := h.module.GetFlamegraph(r.Context(), coretypes.Param(r, "traceId"), req.SelectedSpanID, req.SelectFields)
	if err != nil {
		render.Error(rw, err)
		return
	}

	render.Success(rw, http.StatusOK, result)
}
