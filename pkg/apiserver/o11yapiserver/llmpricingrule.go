package o11yapiserver

import (
	"net/http"

	"github.com/hanzoai/o11y/pkg/http/handler"
	"github.com/hanzoai/o11y/pkg/http/routing"
	"github.com/hanzoai/o11y/pkg/types"
	"github.com/hanzoai/o11y/pkg/types/llmpricingruletypes"
)

// The four pricing-rule routes are ALSO declared as typed ops at the module's
// mount seam (llmobs.go in the repo root), which is what carries them into the
// composed document, the SDK, the CLI and the agent surface. That is a second
// DISPATCH, never a second implementation: the ops answer by handing the call to
// this router, so the handlers below stay the one place the work is performed —
// and the gates declared here (ViewAccess, AdminAccess) stay the one place
// access is decided. Both halves are needed — the ops serve the composed binary,
// this router serves the standalone process, which has no native router to
// register an op on — so deleting either drops one of the two deployments.
func (provider *provider) addLLMPricingRuleRoutes(router routing.Router) {
	router.Get("/v1/o11y/llm_pricing_rules", handler.New(
		provider.authzMiddleware.ViewAccess(provider.llmPricingRuleHandler.List),
		handler.OpenAPIDef{
			ID:                  "ListLLMPricingRules",
			Tags:                []string{"llmpricingrules"},
			Summary:             "List pricing rules",
			Description:         "Returns all LLM pricing rules for the authenticated org, with pagination.",
			Request:             nil,
			RequestContentType:  "",
			RequestQuery:        new(llmpricingruletypes.ListPricingRulesQuery),
			Response:            new(llmpricingruletypes.GettablePricingRules),
			ResponseContentType: "application/json",
			SuccessStatusCode:   http.StatusOK,
			ErrorStatusCodes:    []int{http.StatusBadRequest},
			Deprecated:          false,
			SecuritySchemes:     newSecuritySchemes(types.RoleViewer),
		},
	))

	router.Put("/v1/o11y/llm_pricing_rules", handler.New(
		provider.authzMiddleware.AdminAccess(provider.llmPricingRuleHandler.CreateOrUpdate),
		handler.OpenAPIDef{
			ID:                 "CreateOrUpdateLLMPricingRules",
			Tags:               []string{"llmpricingrules"},
			Summary:            "Create or update pricing rules",
			Description:        "Single write endpoint used by both the user and the Zeus sync job. Per-rule match is by id, then sourceId, then insert. Override rows (is_override=true) are fully preserved when the request does not provide isOverride; only synced_at is stamped.",
			Request:            new(llmpricingruletypes.UpdatableLLMPricingRules),
			RequestContentType: "application/json",
			SuccessStatusCode:  http.StatusNoContent,
			ErrorStatusCodes:   []int{http.StatusBadRequest},
			Deprecated:         false,
			SecuritySchemes:    newSecuritySchemes(types.RoleAdmin),
		},
	))

	router.Get("/v1/o11y/llm_pricing_rules/{id}", handler.New(
		provider.authzMiddleware.ViewAccess(provider.llmPricingRuleHandler.Get),
		handler.OpenAPIDef{
			ID:                  "GetLLMPricingRule",
			Tags:                []string{"llmpricingrules"},
			Summary:             "Get a pricing rule",
			Description:         "Returns a single LLM pricing rule by ID.",
			Request:             nil,
			RequestContentType:  "",
			Response:            new(llmpricingruletypes.GettableLLMPricingRule),
			ResponseContentType: "application/json",
			SuccessStatusCode:   http.StatusOK,
			ErrorStatusCodes:    []int{http.StatusNotFound},
			Deprecated:          false,
			SecuritySchemes:     newSecuritySchemes(types.RoleViewer),
		},
	))

	router.Delete("/v1/o11y/llm_pricing_rules/{id}", handler.New(
		provider.authzMiddleware.AdminAccess(provider.llmPricingRuleHandler.Delete),
		handler.OpenAPIDef{
			ID:                  "DeleteLLMPricingRule",
			Tags:                []string{"llmpricingrules"},
			Summary:             "Delete a pricing rule",
			Description:         "Hard-deletes a pricing rule. If auto-synced, it will be recreated on the next sync cycle.",
			Request:             nil,
			RequestContentType:  "",
			Response:            nil,
			ResponseContentType: "",
			SuccessStatusCode:   http.StatusNoContent,
			ErrorStatusCodes:    []int{http.StatusNotFound},
			Deprecated:          false,
			SecuritySchemes:     newSecuritySchemes(types.RoleAdmin),
		},
	))
}
