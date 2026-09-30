package o11yapiserver

import (
	"net/http"

	"github.com/hanzoai/o11y/pkg/http/handler"
	"github.com/hanzoai/o11y/pkg/http/routing"
	"github.com/hanzoai/o11y/pkg/types"
	"github.com/hanzoai/o11y/pkg/types/errortrackingtypes"
	"github.com/hanzoai/o11y/pkg/types/sentrytypes"
)

// addSentryRoutes registers the FACE, /v1/o11y/sentinel — Sentinel, the
// Sentry-parity product read by a signed-in person: projects, issues, discover,
// events, logs, traces, stats. Hanzo IAM authz, every one scoped to the caller's
// org from the validated claims. There is no ingest here: errors enter through
// /v1/event only.
//
// These are literal paths on the SAME router the o11y read plane uses, so no
// rewrite applies (see createPublicServer: the path that arrives is the path that
// matches). The module's mount seam declares them again as typed ops
// (sentryerrors.go, telemetry.go) that dispatch into this router, so the handlers
// below stay the one place the reads are performed.
func (provider *provider) addSentryRoutes(router routing.Router) {
	h := provider.sentryHandler

	staticRoutes := []struct {
		method string
		path   string
		fn     http.HandlerFunc
		def    handler.OpenAPIDef
	}{
		{http.MethodGet, "/v1/o11y/sentinel/projects", provider.authzMiddleware.ViewAccess(h.ListProjects), handler.OpenAPIDef{
			ID: "SentryListProjects", Tags: []string{"sentry"}, Summary: "List Sentry projects",
			Response: new(sentrytypes.GettableProjects), ResponseContentType: "application/json",
			SuccessStatusCode: http.StatusOK, SecuritySchemes: newSecuritySchemes(types.RoleViewer),
		}},
		{http.MethodPost, "/v1/o11y/sentinel/projects", provider.authzMiddleware.EditAccess(h.CreateProject), handler.OpenAPIDef{
			ID: "SentryCreateProject", Tags: []string{"sentry"}, Summary: "Create a Sentry project",
			Request: new(sentrytypes.PostableProject), RequestContentType: "application/json",
			Response: new(sentrytypes.GettableProject), ResponseContentType: "application/json",
			SuccessStatusCode: http.StatusOK, ErrorStatusCodes: []int{http.StatusBadRequest},
			SecuritySchemes: newSecuritySchemes(types.RoleEditor),
		}},
		{http.MethodGet, "/v1/o11y/sentinel/projects/{id}", provider.authzMiddleware.ViewAccess(h.GetProject), handler.OpenAPIDef{
			ID: "SentryGetProject", Tags: []string{"sentry"}, Summary: "Get a Sentry project",
			Response: new(sentrytypes.GettableProject), ResponseContentType: "application/json",
			SuccessStatusCode: http.StatusOK, ErrorStatusCodes: []int{http.StatusNotFound},
			SecuritySchemes: newSecuritySchemes(types.RoleViewer),
		}},
		{http.MethodDelete, "/v1/o11y/sentinel/projects/{id}", provider.authzMiddleware.EditAccess(h.DeleteProject), handler.OpenAPIDef{
			ID: "SentryDeleteProject", Tags: []string{"sentry"}, Summary: "Delete a Sentry project",
			SuccessStatusCode: http.StatusNoContent, ErrorStatusCodes: []int{http.StatusNotFound},
			SecuritySchemes: newSecuritySchemes(types.RoleEditor),
		}},
		{http.MethodGet, "/v1/o11y/sentinel/issues", provider.authzMiddleware.ViewAccess(h.ListIssues), handler.OpenAPIDef{
			ID: "SentryListIssues", Tags: []string{"sentry"}, Summary: "List error issues",
			RequestQuery: new(errortrackingtypes.IssuesQuery), Response: new(errortrackingtypes.GettableIssues),
			ResponseContentType: "application/json", SuccessStatusCode: http.StatusOK,
			SecuritySchemes: newSecuritySchemes(types.RoleViewer),
		}},
		{http.MethodGet, "/v1/o11y/sentinel/issues/{id}", provider.authzMiddleware.ViewAccess(h.GetIssue), handler.OpenAPIDef{
			ID: "SentryGetIssue", Tags: []string{"sentry"}, Summary: "Get an error issue",
			Response: new(errortrackingtypes.GettableIssue), ResponseContentType: "application/json",
			SuccessStatusCode: http.StatusOK, ErrorStatusCodes: []int{http.StatusNotFound},
			SecuritySchemes: newSecuritySchemes(types.RoleViewer),
		}},
		{http.MethodPut, "/v1/o11y/sentinel/issues/{id}", provider.authzMiddleware.EditAccess(h.UpdateIssue), handler.OpenAPIDef{
			ID: "SentryUpdateIssue", Tags: []string{"sentry"}, Summary: "Update an issue's lifecycle",
			Request: new(errortrackingtypes.UpdateIssue), RequestContentType: "application/json",
			Response: new(errortrackingtypes.Issue), ResponseContentType: "application/json",
			SuccessStatusCode: http.StatusOK, ErrorStatusCodes: []int{http.StatusBadRequest, http.StatusNotFound},
			SecuritySchemes: newSecuritySchemes(types.RoleEditor),
		}},
		{http.MethodGet, "/v1/o11y/sentinel/issues/{id}/events", provider.authzMiddleware.ViewAccess(h.IssueEvents), handler.OpenAPIDef{
			ID: "SentryIssueEvents", Tags: []string{"sentry"}, Summary: "List an issue's occurrences",
			ResponseContentType: "application/json", SuccessStatusCode: http.StatusOK,
			SecuritySchemes: newSecuritySchemes(types.RoleViewer),
		}},
		{http.MethodPost, "/v1/o11y/sentinel/discover", provider.authzMiddleware.ViewAccess(h.Discover), handler.OpenAPIDef{
			ID: "SentryDiscover", Tags: []string{"sentry"}, Summary: "Query the events plane",
			Request: new(sentrytypes.DiscoverRequest), RequestContentType: "application/json",
			Response: new(sentrytypes.DiscoverResult), ResponseContentType: "application/json",
			SuccessStatusCode: http.StatusOK, ErrorStatusCodes: []int{http.StatusBadRequest},
			SecuritySchemes: newSecuritySchemes(types.RoleViewer),
		}},
		{http.MethodGet, "/v1/o11y/sentinel/events/{id}", provider.authzMiddleware.ViewAccess(h.GetEvent), handler.OpenAPIDef{
			ID: "SentryGetEvent", Tags: []string{"sentry"}, Summary: "Get one error event",
			Response: new(sentrytypes.Event), ResponseContentType: "application/json",
			SuccessStatusCode: http.StatusOK, ErrorStatusCodes: []int{http.StatusNotFound},
			SecuritySchemes: newSecuritySchemes(types.RoleViewer),
		}},
		{http.MethodGet, "/v1/o11y/sentinel/logs", provider.authzMiddleware.ViewAccess(h.ListLogs), handler.OpenAPIDef{
			ID: "SentryListLogs", Tags: []string{"sentry"}, Summary: "List error-event logs",
			ResponseContentType: "application/json", SuccessStatusCode: http.StatusOK,
			ErrorStatusCodes: []int{http.StatusBadRequest}, SecuritySchemes: newSecuritySchemes(types.RoleViewer),
		}},
		{http.MethodGet, "/v1/o11y/sentinel/traces", provider.authzMiddleware.ViewAccess(h.ListTraces), handler.OpenAPIDef{
			ID: "SentryListTraces", Tags: []string{"sentry"}, Summary: "List error-correlated traces",
			ResponseContentType: "application/json", SuccessStatusCode: http.StatusOK,
			ErrorStatusCodes: []int{http.StatusBadRequest}, SecuritySchemes: newSecuritySchemes(types.RoleViewer),
		}},
		{http.MethodGet, "/v1/o11y/sentinel/traces/{id}", provider.authzMiddleware.ViewAccess(h.GetTrace), handler.OpenAPIDef{
			ID: "SentryGetTrace", Tags: []string{"sentry"}, Summary: "Get a trace waterfall",
			ResponseContentType: "application/json", SuccessStatusCode: http.StatusOK,
			ErrorStatusCodes: []int{http.StatusBadRequest, http.StatusNotFound}, SecuritySchemes: newSecuritySchemes(types.RoleViewer),
		}},
		{http.MethodGet, "/v1/o11y/sentinel/stats", provider.authzMiddleware.ViewAccess(h.Stats), handler.OpenAPIDef{
			ID: "SentryStats", Tags: []string{"sentry"}, Summary: "Event-rate timeseries",
			ResponseContentType: "application/json", SuccessStatusCode: http.StatusOK,
			ErrorStatusCodes: []int{http.StatusBadRequest}, SecuritySchemes: newSecuritySchemes(types.RoleViewer),
		}},
	}
	for _, rt := range staticRoutes {
		router.Handle(rt.method, rt.path, handler.New(rt.fn, rt.def))
	}

}
