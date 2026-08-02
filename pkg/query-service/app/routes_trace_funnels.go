package app

import (
	"net/http"

	"github.com/hanzoai/o11y/pkg/http/middleware"
	"github.com/hanzoai/o11y/pkg/http/routing"
)

// mountTraceFunnels registers funnel CRUD plus the two analytics families
// (saved funnel by id, and ad-hoc funnel supplied in the payload).
// 18 routes on its own /v1/o11y/trace-funnels subrouter.
//
// ORDER IS LOAD-BEARING: the literal children /new, /list and /steps/update are
// registered before /{funnel_id}, so GET /list is the list handler and not a
// funnel lookup with funnel_id="list". Preserved verbatim.
func (aH *APIHandler) mountTraceFunnels(router routing.Router, am *middleware.AuthZ) {
	// Main trace funnels router
	traceFunnelsRouter := router.Group("/v1/o11y/trace-funnels")

	// API endpoints
	traceFunnelsRouter.Post("/new",
		am.EditAccess(aH.O11y.Handlers.TraceFunnel.New))
	traceFunnelsRouter.Get("/list",
		am.ViewAccess(aH.O11y.Handlers.TraceFunnel.List))
	traceFunnelsRouter.Put("/steps/update",
		am.EditAccess(aH.O11y.Handlers.TraceFunnel.UpdateSteps))

	traceFunnelsRouter.Get("/{funnel_id}",
		am.ViewAccess(aH.O11y.Handlers.TraceFunnel.Get))
	traceFunnelsRouter.Delete("/{funnel_id}",
		am.EditAccess(aH.O11y.Handlers.TraceFunnel.Delete))
	traceFunnelsRouter.Put("/{funnel_id}",
		am.EditAccess(aH.O11y.Handlers.TraceFunnel.UpdateFunnel))

	// Analytics endpoints
	traceFunnelsRouter.Post("/{funnel_id}/analytics/validate", http.HandlerFunc(aH.handleValidateTraces))
	traceFunnelsRouter.Post("/{funnel_id}/analytics/overview", http.HandlerFunc(aH.handleFunnelAnalytics))
	traceFunnelsRouter.Post("/{funnel_id}/analytics/steps", http.HandlerFunc(aH.handleStepAnalytics))
	traceFunnelsRouter.Post("/{funnel_id}/analytics/steps/overview", http.HandlerFunc(aH.handleFunnelStepAnalytics))
	traceFunnelsRouter.Post("/{funnel_id}/analytics/slow-traces", http.HandlerFunc(aH.handleFunnelSlowTraces))
	traceFunnelsRouter.Post("/{funnel_id}/analytics/error-traces", http.HandlerFunc(aH.handleFunnelErrorTraces))

	// Analytics endpoints
	traceFunnelsRouter.Post("/analytics/validate", http.HandlerFunc(aH.handleValidateTracesWithPayload))
	traceFunnelsRouter.Post("/analytics/overview", http.HandlerFunc(aH.handleFunnelAnalyticsWithPayload))
	traceFunnelsRouter.Post("/analytics/steps", http.HandlerFunc(aH.handleStepAnalyticsWithPayload))
	traceFunnelsRouter.Post("/analytics/steps/overview", http.HandlerFunc(aH.handleFunnelStepAnalyticsWithPayload))
	traceFunnelsRouter.Post("/analytics/slow-traces", http.HandlerFunc(aH.handleFunnelSlowTracesWithPayload))
	traceFunnelsRouter.Post("/analytics/error-traces", http.HandlerFunc(aH.handleFunnelErrorTracesWithPayload))
}
