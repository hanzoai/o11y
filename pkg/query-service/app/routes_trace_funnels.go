package app

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/hanzoai/o11y/pkg/http/middleware"
)

// mountTraceFunnels registers funnel CRUD plus the two analytics families
// (saved funnel by id, and ad-hoc funnel supplied in the payload).
// 18 routes on its own /v1/o11y/trace-funnels subrouter.
//
// ORDER IS LOAD-BEARING: the literal children /new, /list and /steps/update are
// registered before /{funnel_id}, so GET /list is the list handler and not a
// funnel lookup with funnel_id="list". Preserved verbatim.
func (aH *APIHandler) mountTraceFunnels(router *mux.Router, am *middleware.AuthZ) {
	// Main trace funnels router
	traceFunnelsRouter := router.PathPrefix("/v1/o11y/trace-funnels").Subrouter()

	// API endpoints
	traceFunnelsRouter.HandleFunc("/new",
		am.EditAccess(aH.O11y.Handlers.TraceFunnel.New)).
		Methods(http.MethodPost)
	traceFunnelsRouter.HandleFunc("/list",
		am.ViewAccess(aH.O11y.Handlers.TraceFunnel.List)).
		Methods(http.MethodGet)
	traceFunnelsRouter.HandleFunc("/steps/update",
		am.EditAccess(aH.O11y.Handlers.TraceFunnel.UpdateSteps)).
		Methods(http.MethodPut)

	traceFunnelsRouter.HandleFunc("/{funnel_id}",
		am.ViewAccess(aH.O11y.Handlers.TraceFunnel.Get)).
		Methods(http.MethodGet)
	traceFunnelsRouter.HandleFunc("/{funnel_id}",
		am.EditAccess(aH.O11y.Handlers.TraceFunnel.Delete)).
		Methods(http.MethodDelete)
	traceFunnelsRouter.HandleFunc("/{funnel_id}",
		am.EditAccess(aH.O11y.Handlers.TraceFunnel.UpdateFunnel)).
		Methods(http.MethodPut)

	// Analytics endpoints
	traceFunnelsRouter.HandleFunc("/{funnel_id}/analytics/validate", aH.handleValidateTraces).Methods("POST")
	traceFunnelsRouter.HandleFunc("/{funnel_id}/analytics/overview", aH.handleFunnelAnalytics).Methods("POST")
	traceFunnelsRouter.HandleFunc("/{funnel_id}/analytics/steps", aH.handleStepAnalytics).Methods("POST")
	traceFunnelsRouter.HandleFunc("/{funnel_id}/analytics/steps/overview", aH.handleFunnelStepAnalytics).Methods("POST")
	traceFunnelsRouter.HandleFunc("/{funnel_id}/analytics/slow-traces", aH.handleFunnelSlowTraces).Methods("POST")
	traceFunnelsRouter.HandleFunc("/{funnel_id}/analytics/error-traces", aH.handleFunnelErrorTraces).Methods("POST")

	// Analytics endpoints
	traceFunnelsRouter.HandleFunc("/analytics/validate", aH.handleValidateTracesWithPayload).Methods("POST")
	traceFunnelsRouter.HandleFunc("/analytics/overview", aH.handleFunnelAnalyticsWithPayload).Methods("POST")
	traceFunnelsRouter.HandleFunc("/analytics/steps", aH.handleStepAnalyticsWithPayload).Methods("POST")
	traceFunnelsRouter.HandleFunc("/analytics/steps/overview", aH.handleFunnelStepAnalyticsWithPayload).Methods("POST")
	traceFunnelsRouter.HandleFunc("/analytics/slow-traces", aH.handleFunnelSlowTracesWithPayload).Methods("POST")
	traceFunnelsRouter.HandleFunc("/analytics/error-traces", aH.handleFunnelErrorTracesWithPayload).Methods("POST")
}
