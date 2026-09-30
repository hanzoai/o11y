package o11yapiserver

import (
	"net/http"

	"github.com/hanzoai/o11y/pkg/http/handler"
	"github.com/hanzoai/o11y/pkg/http/routing"
	"github.com/hanzoai/o11y/pkg/types"
	"github.com/hanzoai/o11y/pkg/types/errortrackingtypes"
)

// addErrorTrackingRoutes serves the Issues list/detail/update the console Errors tab
// consumes at /v1/o11y/errortracking/issues[/{id}]: Hanzo IAM authz, org-scoped.
// Errors enter through /v1/event only, so there is no ingest here.
//
// The three routes are ALSO typed ops at the module's mount seam (sentryerrors.go,
// on the /v1/o11y group), which is what carries them into the document, the SDK,
// the CLI and the agent surface. That is a second DISPATCH, never a second
// implementation: the ops answer by handing the call to THIS router.
func (provider *provider) addErrorTrackingRoutes(router routing.Router) {
	h := provider.errorTrackingHandler

	routes := []struct {
		method string
		path   string
		fn     http.HandlerFunc
		def    handler.OpenAPIDef
	}{
		{http.MethodGet, "/v1/o11y/errortracking/issues", provider.authzMiddleware.ViewAccess(h.ListIssues), handler.OpenAPIDef{
			ID: "ListIssues", Tags: []string{"errortracking"}, Summary: "List error issues",
			Description:         "Lists grouped error issues (by fingerprint) for the caller's org with status, level, counts and first/last-seen.",
			RequestQuery:        new(errortrackingtypes.IssuesQuery),
			Response:            new(errortrackingtypes.GettableIssues),
			ResponseContentType: "application/json", SuccessStatusCode: http.StatusOK,
			ErrorStatusCodes: []int{http.StatusBadRequest}, SecuritySchemes: newSecuritySchemes(types.RoleViewer),
		}},
		{http.MethodGet, "/v1/o11y/errortracking/issues/{id}", provider.authzMiddleware.ViewAccess(h.GetIssue), handler.OpenAPIDef{
			ID: "GetIssue", Tags: []string{"errortracking"}, Summary: "Get an error issue",
			Description:         "Returns a single issue with its latest occurrence sample.",
			Response:            new(errortrackingtypes.GettableIssue),
			ResponseContentType: "application/json", SuccessStatusCode: http.StatusOK,
			ErrorStatusCodes: []int{http.StatusNotFound}, SecuritySchemes: newSecuritySchemes(types.RoleViewer),
		}},
		{http.MethodPost, "/v1/o11y/errortracking/issues/{id}", provider.authzMiddleware.EditAccess(h.UpdateIssue), handler.OpenAPIDef{
			ID: "UpdateIssue", Tags: []string{"errortracking"}, Summary: "Update an issue's lifecycle",
			Description: "Resolve, ignore, reopen or assign an issue.",
			Request:     new(errortrackingtypes.UpdateIssue), RequestContentType: "application/json",
			Response:            new(errortrackingtypes.Issue),
			ResponseContentType: "application/json", SuccessStatusCode: http.StatusOK,
			ErrorStatusCodes: []int{http.StatusBadRequest, http.StatusNotFound}, SecuritySchemes: newSecuritySchemes(types.RoleEditor),
		}},
	}

	for _, rt := range routes {
		router.Handle(rt.method, rt.path, handler.New(rt.fn, rt.def))
	}
}
