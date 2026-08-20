package o11yapiserver

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/hanzoai/o11y/pkg/http/handler"
	"github.com/hanzoai/o11y/pkg/types"
	"github.com/hanzoai/o11y/pkg/types/errortrackingtypes"
)

// addErrorTrackingRoutes serves the grouped-issue READ surface (Hanzo IAM authz,
// org-scoped): the Issues list/detail/update the console Errors tab consumes at
// /v1/o11y/errortracking/issues[/{id}].
//
// There is no ingest route here. Errors enter through the ONE ingest face,
// /v1/sentry/{project}/envelope|store, which writes event.error and folds the batch
// into these issues — one writer per table.
func (provider *provider) addErrorTrackingRoutes(router *mux.Router) error {
	h := provider.errorTrackingHandler

	routes := []struct {
		method string
		path   string
		fn     http.HandlerFunc
		def    handler.OpenAPIDef
	}{
		{http.MethodGet, "/api/errortracking/issues", provider.authzMiddleware.ViewAccess(h.ListIssues), handler.OpenAPIDef{
			ID: "ListIssues", Tags: []string{"errortracking"}, Summary: "List error issues",
			Description:         "Lists grouped error issues (by fingerprint) for the caller's org with status, level, counts and first/last-seen.",
			RequestQuery:        new(errortrackingtypes.IssuesQuery),
			Response:            new(errortrackingtypes.GettableIssues),
			ResponseContentType: "application/json", SuccessStatusCode: http.StatusOK,
			ErrorStatusCodes: []int{http.StatusBadRequest}, SecuritySchemes: newSecuritySchemes(types.RoleViewer),
		}},
		{http.MethodGet, "/api/errortracking/issues/{id}", provider.authzMiddleware.ViewAccess(h.GetIssue), handler.OpenAPIDef{
			ID: "GetIssue", Tags: []string{"errortracking"}, Summary: "Get an error issue",
			Description:         "Returns a single issue with its latest occurrence sample.",
			Response:            new(errortrackingtypes.GettableIssue),
			ResponseContentType: "application/json", SuccessStatusCode: http.StatusOK,
			ErrorStatusCodes: []int{http.StatusNotFound}, SecuritySchemes: newSecuritySchemes(types.RoleViewer),
		}},
		{http.MethodPost, "/api/errortracking/issues/{id}", provider.authzMiddleware.EditAccess(h.UpdateIssue), handler.OpenAPIDef{
			ID: "UpdateIssue", Tags: []string{"errortracking"}, Summary: "Update an issue's lifecycle",
			Description: "Resolve, ignore, reopen or assign an issue.",
			Request:     new(errortrackingtypes.UpdateIssue), RequestContentType: "application/json",
			Response:            new(errortrackingtypes.Issue),
			ResponseContentType: "application/json", SuccessStatusCode: http.StatusOK,
			ErrorStatusCodes: []int{http.StatusBadRequest, http.StatusNotFound}, SecuritySchemes: newSecuritySchemes(types.RoleEditor),
		}},
	}

	for _, rt := range routes {
		if err := router.Handle(rt.path, handler.New(rt.fn, rt.def)).Methods(rt.method).GetError(); err != nil {
			return err
		}
	}

	return nil
}
