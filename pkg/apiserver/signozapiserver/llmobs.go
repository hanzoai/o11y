package signozapiserver

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/hanzoai/o11y/pkg/http/handler"
	"github.com/hanzoai/o11y/pkg/types"
	"github.com/hanzoai/o11y/pkg/types/llmobstypes"
)

// addLLMObsRoutes serves the native LLM-observability surface (absorbing
// the upstream product) under /v1/o11y. Observations/traces/sessions/users are views over
// gen_ai spans; scores and annotations are CRUD over net-new tables. Every
// route is behind the shared Hanzo IAM authz middleware.
func (provider *provider) addLLMObsRoutes(router *mux.Router) error {
	h := provider.llmObsHandler

	routes := []struct {
		method string
		path   string
		fn     http.HandlerFunc
		def    handler.OpenAPIDef
	}{
		{http.MethodGet, "/api/observations", provider.authzMiddleware.ViewAccess(h.Observations), handler.OpenAPIDef{
			ID: "ListLLMObservations", Tags: []string{"llmobs"}, Summary: "List observations",
			Description:         "Lists gen_ai spans (LLM observations) with model, tokens, cost and latency projected from gen_ai.* attributes.",
			RequestQuery:        new(llmobstypes.ViewQuery),
			Response:            new(llmobstypes.GettableObservations),
			ResponseContentType: "application/json", SuccessStatusCode: http.StatusOK,
			ErrorStatusCodes: []int{http.StatusBadRequest}, SecuritySchemes: newSecuritySchemes(types.RoleViewer),
		}},
		{http.MethodGet, "/api/traces", provider.authzMiddleware.ViewAccess(h.Traces), handler.OpenAPIDef{
			ID: "ListLLMTraces", Tags: []string{"llmobs"}, Summary: "List traces",
			Description:         "Lists LLM traces (gen_ai spans grouped by trace_id) with rolled-up cost, tokens and latency.",
			RequestQuery:        new(llmobstypes.ViewQuery),
			Response:            new(llmobstypes.GettableTraces),
			ResponseContentType: "application/json", SuccessStatusCode: http.StatusOK,
			ErrorStatusCodes: []int{http.StatusBadRequest}, SecuritySchemes: newSecuritySchemes(types.RoleViewer),
		}},
		{http.MethodGet, "/api/sessions", provider.authzMiddleware.ViewAccess(h.Sessions), handler.OpenAPIDef{
			ID: "ListLLMSessions", Tags: []string{"llmobs"}, Summary: "List sessions",
			Description:         "Lists conversations (gen_ai spans grouped by session.id) with trace/observation counts, tokens and cost.",
			RequestQuery:        new(llmobstypes.ViewQuery),
			Response:            new(llmobstypes.GettableSessions),
			ResponseContentType: "application/json", SuccessStatusCode: http.StatusOK,
			ErrorStatusCodes: []int{http.StatusBadRequest}, SecuritySchemes: newSecuritySchemes(types.RoleViewer),
		}},
		{http.MethodGet, "/api/users", provider.authzMiddleware.ViewAccess(h.Users), handler.OpenAPIDef{
			ID: "ListLLMUsers", Tags: []string{"llmobs"}, Summary: "List users",
			Description:         "Lists end users (gen_ai spans grouped by user.id) with session/trace/observation counts, tokens and cost.",
			RequestQuery:        new(llmobstypes.ViewQuery),
			Response:            new(llmobstypes.GettableUsers),
			ResponseContentType: "application/json", SuccessStatusCode: http.StatusOK,
			ErrorStatusCodes: []int{http.StatusBadRequest}, SecuritySchemes: newSecuritySchemes(types.RoleViewer),
		}},
		{http.MethodGet, "/api/scores", provider.authzMiddleware.ViewAccess(h.ListScores), handler.OpenAPIDef{
			ID: "ListLLMScores", Tags: []string{"llmobs"}, Summary: "List scores",
			Description:         "Lists eval scores and human-feedback signals attached to traces/observations.",
			RequestQuery:        new(llmobstypes.ScoresQuery),
			Response:            new(llmobstypes.GettableScores),
			ResponseContentType: "application/json", SuccessStatusCode: http.StatusOK,
			ErrorStatusCodes: []int{http.StatusBadRequest}, SecuritySchemes: newSecuritySchemes(types.RoleViewer),
		}},
		{http.MethodPost, "/api/scores", provider.authzMiddleware.EditAccess(h.CreateScore), handler.OpenAPIDef{
			ID: "CreateLLMScore", Tags: []string{"llmobs"}, Summary: "Create a score",
			Description: "Attaches an eval score or human-feedback signal to a trace or observation.",
			Request:     new(llmobstypes.IngestScore), RequestContentType: "application/json",
			Response:            new(llmobstypes.Score),
			ResponseContentType: "application/json", SuccessStatusCode: http.StatusCreated,
			ErrorStatusCodes: []int{http.StatusBadRequest}, SecuritySchemes: newSecuritySchemes(types.RoleEditor),
		}},
		{http.MethodGet, "/api/score/{id}", provider.authzMiddleware.ViewAccess(h.GetScore), handler.OpenAPIDef{
			ID: "GetLLMScore", Tags: []string{"llmobs"}, Summary: "Get a score",
			Description:         "Returns a single score by id.",
			Response:            new(llmobstypes.Score),
			ResponseContentType: "application/json", SuccessStatusCode: http.StatusOK,
			ErrorStatusCodes: []int{http.StatusNotFound}, SecuritySchemes: newSecuritySchemes(types.RoleViewer),
		}},
		{http.MethodDelete, "/api/score/{id}", provider.authzMiddleware.EditAccess(h.DeleteScore), handler.OpenAPIDef{
			ID: "DeleteLLMScore", Tags: []string{"llmobs"}, Summary: "Delete a score",
			Description:       "Hard-deletes a score by id.",
			SuccessStatusCode: http.StatusNoContent,
			ErrorStatusCodes:  []int{http.StatusNotFound}, SecuritySchemes: newSecuritySchemes(types.RoleEditor),
		}},
		{http.MethodGet, "/api/annotation", provider.authzMiddleware.ViewAccess(h.Annotations), handler.OpenAPIDef{
			ID: "ListLLMAnnotations", Tags: []string{"llmobs"}, Summary: "List annotations",
			Description:         "Lists human annotations (optionally scoped to a review queue) on traces/observations.",
			RequestQuery:        new(llmobstypes.AnnotationsQuery),
			Response:            new(llmobstypes.GettableAnnotations),
			ResponseContentType: "application/json", SuccessStatusCode: http.StatusOK,
			ErrorStatusCodes: []int{http.StatusBadRequest}, SecuritySchemes: newSecuritySchemes(types.RoleViewer),
		}},
		{http.MethodPost, "/api/annotation", provider.authzMiddleware.EditAccess(h.CreateAnnotation), handler.OpenAPIDef{
			ID: "CreateLLMAnnotation", Tags: []string{"llmobs"}, Summary: "Create an annotation",
			Description: "Adds a human annotation to a trace or observation, optionally in a review queue.",
			Request:     new(llmobstypes.IngestAnnotation), RequestContentType: "application/json",
			Response:            new(llmobstypes.Annotation),
			ResponseContentType: "application/json", SuccessStatusCode: http.StatusCreated,
			ErrorStatusCodes: []int{http.StatusBadRequest}, SecuritySchemes: newSecuritySchemes(types.RoleEditor),
		}},
	}

	for _, rt := range routes {
		if err := router.Handle(rt.path, handler.New(rt.fn, rt.def)).Methods(rt.method).GetError(); err != nil {
			return err
		}
	}

	return nil
}
