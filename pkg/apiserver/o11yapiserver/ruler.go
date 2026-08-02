package o11yapiserver

import (
	"net/http"

	"github.com/hanzoai/o11y/pkg/http/handler"
	"github.com/hanzoai/o11y/pkg/http/routing"
	"github.com/hanzoai/o11y/pkg/types"
	"github.com/hanzoai/o11y/pkg/types/alertmanagertypes"
	"github.com/hanzoai/o11y/pkg/types/ruletypes"
)

// ALL of these routes are ALSO declared as typed ops at the module's mount seam
// (rulesalerts.go in the repo root), which is what carries them into the
// composed document, the SDK, the CLI and the agent surface. That is a second
// DISPATCH, never a second implementation: the ops answer by handing the call
// back to this router, so the handlers below stay the one place the work is
// performed — and the gates declared here (ViewAccess, EditAccess) stay the one
// place access is decided. Both halves are needed — the ops serve the composed
// binary, this router serves the standalone process, which has no native router
// to register an op on — so deleting either drops one of the two deployments.
func (provider *provider) addRulerRoutes(router routing.Router) {
	router.Get("/v1/o11y/rules", handler.New(provider.authzMiddleware.ViewAccess(provider.rulerHandler.ListRules), handler.OpenAPIDef{
		ID:                  "ListRules",
		Tags:                []string{"rules"},
		Summary:             "List alert rules",
		Description:         "This endpoint lists all alert rules with their current evaluation state",
		Response:            make([]*ruletypes.Rule, 0),
		ResponseContentType: "application/json",
		SuccessStatusCode:   http.StatusOK,
		SecuritySchemes:     newSecuritySchemes(types.RoleViewer),
	}))

	router.Get("/v1/o11y/rules/{id}", handler.New(provider.authzMiddleware.ViewAccess(provider.rulerHandler.GetRuleByID), handler.OpenAPIDef{
		ID:                  "GetRuleByID",
		Tags:                []string{"rules"},
		Summary:             "Get alert rule by ID",
		Description:         "This endpoint returns an alert rule by ID",
		Response:            new(ruletypes.Rule),
		ResponseContentType: "application/json",
		SuccessStatusCode:   http.StatusOK,
		ErrorStatusCodes:    []int{http.StatusNotFound},
		SecuritySchemes:     newSecuritySchemes(types.RoleViewer),
	}))

	router.Post("/v1/o11y/rules", handler.New(provider.authzMiddleware.EditAccess(provider.rulerHandler.CreateRule), handler.OpenAPIDef{
		ID:                  "CreateRule",
		Tags:                []string{"rules"},
		Summary:             "Create alert rule",
		Description:         "This endpoint creates a new alert rule",
		Request:             new(ruletypes.PostableRule),
		RequestContentType:  "application/json",
		RequestExamples:     postableRuleExamples(),
		Response:            new(ruletypes.Rule),
		ResponseContentType: "application/json",
		SuccessStatusCode:   http.StatusCreated,
		ErrorStatusCodes:    []int{http.StatusBadRequest},
		SecuritySchemes:     newSecuritySchemes(types.RoleEditor),
	}))

	router.Put("/v1/o11y/rules/{id}", handler.New(provider.authzMiddleware.EditAccess(provider.rulerHandler.UpdateRuleByID), handler.OpenAPIDef{
		ID:                 "UpdateRuleByID",
		Tags:               []string{"rules"},
		Summary:            "Update alert rule",
		Description:        "This endpoint updates an alert rule by ID",
		Request:            new(ruletypes.PostableRule),
		RequestContentType: "application/json",
		RequestExamples:    postableRuleExamples(),
		SuccessStatusCode:  http.StatusNoContent,
		ErrorStatusCodes:   []int{http.StatusBadRequest, http.StatusNotFound},
		SecuritySchemes:    newSecuritySchemes(types.RoleEditor),
	}))

	router.Delete("/v1/o11y/rules/{id}", handler.New(provider.authzMiddleware.EditAccess(provider.rulerHandler.DeleteRuleByID), handler.OpenAPIDef{
		ID:                "DeleteRuleByID",
		Tags:              []string{"rules"},
		Summary:           "Delete alert rule",
		Description:       "This endpoint deletes an alert rule by ID",
		SuccessStatusCode: http.StatusNoContent,
		ErrorStatusCodes:  []int{http.StatusNotFound},
		SecuritySchemes:   newSecuritySchemes(types.RoleEditor),
	}))

	router.Patch("/v1/o11y/rules/{id}", handler.New(provider.authzMiddleware.EditAccess(provider.rulerHandler.PatchRuleByID), handler.OpenAPIDef{
		ID:                  "PatchRuleByID",
		Tags:                []string{"rules"},
		Summary:             "Patch alert rule",
		Description:         "This endpoint applies a partial update to an alert rule by ID",
		Request:             new(ruletypes.PostableRule),
		RequestContentType:  "application/json",
		RequestExamples:     postableRuleExamples(),
		Response:            new(ruletypes.Rule),
		ResponseContentType: "application/json",
		SuccessStatusCode:   http.StatusOK,
		ErrorStatusCodes:    []int{http.StatusBadRequest, http.StatusNotFound},
		SecuritySchemes:     newSecuritySchemes(types.RoleEditor),
	}))

	router.Post("/v1/o11y/rules/test", handler.New(provider.authzMiddleware.EditAccess(provider.rulerHandler.TestRule), handler.OpenAPIDef{
		ID:                  "TestRule",
		Tags:                []string{"rules"},
		Summary:             "Test alert rule",
		Description:         "This endpoint fires a test notification for the given rule definition",
		Request:             new(ruletypes.PostableRule),
		RequestContentType:  "application/json",
		RequestExamples:     postableRuleExamples(),
		Response:            new(ruletypes.GettableTestRule),
		ResponseContentType: "application/json",
		SuccessStatusCode:   http.StatusOK,
		ErrorStatusCodes:    []int{http.StatusBadRequest},
		SecuritySchemes:     newSecuritySchemes(types.RoleEditor),
	}))

	router.Get("/v1/o11y/downtime_schedules", handler.New(provider.authzMiddleware.ViewAccess(provider.rulerHandler.ListDowntimeSchedules), handler.OpenAPIDef{
		ID:                  "ListDowntimeSchedules",
		Tags:                []string{"downtimeschedules"},
		Summary:             "List downtime schedules",
		Description:         "This endpoint lists all planned maintenance / downtime schedules",
		RequestQuery:        new(alertmanagertypes.ListPlannedMaintenanceParams),
		Response:            make([]*alertmanagertypes.PlannedMaintenance, 0),
		ResponseContentType: "application/json",
		SuccessStatusCode:   http.StatusOK,
		SecuritySchemes:     newSecuritySchemes(types.RoleViewer),
	}))

	router.Get("/v1/o11y/downtime_schedules/{id}", handler.New(provider.authzMiddleware.ViewAccess(provider.rulerHandler.GetDowntimeScheduleByID), handler.OpenAPIDef{
		ID:                  "GetDowntimeScheduleByID",
		Tags:                []string{"downtimeschedules"},
		Summary:             "Get downtime schedule by ID",
		Description:         "This endpoint returns a downtime schedule by ID",
		Response:            new(alertmanagertypes.PlannedMaintenance),
		ResponseContentType: "application/json",
		SuccessStatusCode:   http.StatusOK,
		ErrorStatusCodes:    []int{http.StatusNotFound},
		SecuritySchemes:     newSecuritySchemes(types.RoleViewer),
	}))

	router.Post("/v1/o11y/downtime_schedules", handler.New(provider.authzMiddleware.EditAccess(provider.rulerHandler.CreateDowntimeSchedule), handler.OpenAPIDef{
		ID:                  "CreateDowntimeSchedule",
		Tags:                []string{"downtimeschedules"},
		Summary:             "Create downtime schedule",
		Description:         "This endpoint creates a new planned maintenance / downtime schedule",
		Request:             new(alertmanagertypes.PostablePlannedMaintenance),
		RequestContentType:  "application/json",
		Response:            new(alertmanagertypes.PlannedMaintenance),
		ResponseContentType: "application/json",
		SuccessStatusCode:   http.StatusCreated,
		ErrorStatusCodes:    []int{http.StatusBadRequest},
		SecuritySchemes:     newSecuritySchemes(types.RoleEditor),
	}))

	router.Put("/v1/o11y/downtime_schedules/{id}", handler.New(provider.authzMiddleware.EditAccess(provider.rulerHandler.UpdateDowntimeScheduleByID), handler.OpenAPIDef{
		ID:                 "UpdateDowntimeScheduleByID",
		Tags:               []string{"downtimeschedules"},
		Summary:            "Update downtime schedule",
		Description:        "This endpoint updates a downtime schedule by ID",
		Request:            new(alertmanagertypes.PostablePlannedMaintenance),
		RequestContentType: "application/json",
		SuccessStatusCode:  http.StatusNoContent,
		ErrorStatusCodes:   []int{http.StatusBadRequest, http.StatusNotFound},
		SecuritySchemes:    newSecuritySchemes(types.RoleEditor),
	}))

	router.Delete("/v1/o11y/downtime_schedules/{id}", handler.New(provider.authzMiddleware.EditAccess(provider.rulerHandler.DeleteDowntimeScheduleByID), handler.OpenAPIDef{
		ID:                "DeleteDowntimeScheduleByID",
		Tags:              []string{"downtimeschedules"},
		Summary:           "Delete downtime schedule",
		Description:       "This endpoint deletes a downtime schedule by ID",
		SuccessStatusCode: http.StatusNoContent,
		ErrorStatusCodes:  []int{http.StatusNotFound},
		SecuritySchemes:   newSecuritySchemes(types.RoleEditor),
	}))
}
