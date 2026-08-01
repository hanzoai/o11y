package o11yapiserver

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/hanzoai/o11y/pkg/http/handler"
	"github.com/hanzoai/o11y/pkg/types"
	"github.com/hanzoai/o11y/pkg/types/rulestatehistorytypes"
	"github.com/hanzoai/o11y/pkg/types/telemetrytypes"
)

// ALL of these routes are ALSO declared as typed ops at the module's mount seam
// (rulesalerts.go in the repo root), which is what carries them into the
// composed document, the SDK, the CLI and the agent surface. That is a second
// DISPATCH, never a second implementation: the ops answer by handing the call
// back to this router, so the handlers below stay the one place the work is
// performed — and the ViewAccess gate declared here stays the one place access
// is decided. Both halves are needed — the ops serve the composed binary, this
// router serves the standalone process, which has no native router to register
// an op on — so deleting either drops one of the two deployments.
func (provider *provider) addRuleStateHistoryRoutes(router *mux.Router) error {

	if err := router.Handle("/v1/o11y/rules/{id}/history/stats", handler.New(
		provider.authzMiddleware.ViewAccess(provider.ruleStateHistoryHandler.GetRuleHistoryStats),
		handler.OpenAPIDef{
			ID:                  "GetRuleHistoryStats",
			Tags:                []string{"rules"},
			Summary:             "Get rule history stats",
			Description:         "Returns trigger and resolution statistics for a rule in the selected time range.",
			RequestQuery:        new(rulestatehistorytypes.PostableRuleStateHistoryBaseQuery),
			Response:            new(rulestatehistorytypes.GettableRuleStateHistoryStats),
			ResponseContentType: "application/json",
			SuccessStatusCode:   http.StatusOK,
			ErrorStatusCodes:    []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusInternalServerError},
			SecuritySchemes:     newSecuritySchemes(types.RoleViewer),
		})).Methods(http.MethodGet).GetError(); err != nil {
		return err
	}

	if err := router.Handle("/v1/o11y/rules/{id}/history/timeline", handler.New(
		provider.authzMiddleware.ViewAccess(provider.ruleStateHistoryHandler.GetRuleHistoryTimeline),
		handler.OpenAPIDef{
			ID:                  "GetRuleHistoryTimeline",
			Tags:                []string{"rules"},
			Summary:             "Get rule history timeline",
			Description:         "Returns paginated timeline entries for rule state transitions.",
			RequestQuery:        new(rulestatehistorytypes.PostableRuleStateHistoryTimelineQuery),
			Response:            new(rulestatehistorytypes.GettableRuleStateTimeline),
			ResponseContentType: "application/json",
			SuccessStatusCode:   http.StatusOK,
			ErrorStatusCodes:    []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusInternalServerError},
			SecuritySchemes:     newSecuritySchemes(types.RoleViewer),
		})).Methods(http.MethodGet).GetError(); err != nil {
		return err
	}

	if err := router.Handle("/v1/o11y/rules/{id}/history/top_contributors", handler.New(
		provider.authzMiddleware.ViewAccess(provider.ruleStateHistoryHandler.GetRuleHistoryContributors),
		handler.OpenAPIDef{
			ID:                  "GetRuleHistoryTopContributors",
			Tags:                []string{"rules"},
			Summary:             "Get top contributors to rule firing",
			Description:         "Returns top label combinations contributing to rule firing in the selected time range.",
			RequestQuery:        new(rulestatehistorytypes.PostableRuleStateHistoryBaseQuery),
			Response:            new([]rulestatehistorytypes.GettableRuleStateHistoryContributor),
			ResponseContentType: "application/json",
			SuccessStatusCode:   http.StatusOK,
			ErrorStatusCodes:    []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusInternalServerError},
			SecuritySchemes:     newSecuritySchemes(types.RoleViewer),
		})).Methods(http.MethodGet).GetError(); err != nil {
		return err
	}

	if err := router.Handle("/v1/o11y/rules/{id}/history/filter_keys", handler.New(
		provider.authzMiddleware.ViewAccess(provider.ruleStateHistoryHandler.GetRuleHistoryFilterKeys),
		handler.OpenAPIDef{
			ID:                  "GetRuleHistoryFilterKeys",
			Tags:                []string{"rules"},
			Summary:             "Get rule history filter keys",
			Description:         "Returns distinct label keys from rule history entries for the selected range.",
			RequestQuery:        new(telemetrytypes.PostableFieldKeysParams),
			Response:            new(telemetrytypes.GettableFieldKeys),
			ResponseContentType: "application/json",
			SuccessStatusCode:   http.StatusOK,
			ErrorStatusCodes:    []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusInternalServerError},
			SecuritySchemes:     newSecuritySchemes(types.RoleViewer),
		})).Methods(http.MethodGet).GetError(); err != nil {
		return err
	}

	if err := router.Handle("/v1/o11y/rules/{id}/history/filter_values", handler.New(
		provider.authzMiddleware.ViewAccess(provider.ruleStateHistoryHandler.GetRuleHistoryFilterValues),
		handler.OpenAPIDef{
			ID:                  "GetRuleHistoryFilterValues",
			Tags:                []string{"rules"},
			Summary:             "Get rule history filter values",
			Description:         "Returns distinct label values for a given key from rule history entries.",
			RequestQuery:        new(telemetrytypes.PostableFieldValueParams),
			Response:            new(telemetrytypes.GettableFieldValues),
			ResponseContentType: "application/json",
			SuccessStatusCode:   http.StatusOK,
			ErrorStatusCodes:    []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusInternalServerError},
			SecuritySchemes:     newSecuritySchemes(types.RoleViewer),
		})).Methods(http.MethodGet).GetError(); err != nil {
		return err
	}

	if err := router.Handle("/v1/o11y/rules/{id}/history/overall_status", handler.New(
		provider.authzMiddleware.ViewAccess(provider.ruleStateHistoryHandler.GetRuleHistoryOverallStatus),
		handler.OpenAPIDef{
			ID:                  "GetRuleHistoryOverallStatus",
			Tags:                []string{"rules"},
			Summary:             "Get rule overall status timeline",
			Description:         "Returns overall firing/inactive intervals for a rule in the selected time range.",
			RequestQuery:        new(rulestatehistorytypes.PostableRuleStateHistoryBaseQuery),
			Response:            new([]rulestatehistorytypes.GettableRuleStateWindow),
			ResponseContentType: "application/json",
			SuccessStatusCode:   http.StatusOK,
			ErrorStatusCodes:    []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusInternalServerError},
			SecuritySchemes:     newSecuritySchemes(types.RoleViewer),
		})).Methods(http.MethodGet).GetError(); err != nil {
		return err
	}

	return nil
}
