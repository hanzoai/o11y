package o11yapiserver

import (
	"net/http"

	"github.com/hanzoai/o11y/pkg/http/handler"
	"github.com/hanzoai/o11y/pkg/http/routing"
	"github.com/hanzoai/o11y/pkg/types"
	"github.com/hanzoai/o11y/pkg/types/errortrackingtypes"
)

// addErrorTrackingRoutes serves error/crash tracking (Sentry-class Issues) under
// /v1/o11y. Two families:
//
//   - INGEST (public, DSN-authenticated in-handler): the Sentry wire endpoints
//     POST /v1/o11y/api/{project}/envelope/ and POST /v1/o11y/api/{project}/store/.
//     They are wrapped with OpenAccess (no IAM) because the Sentry SDK presents a DSN
//     key, not a Hanzo session; the handler verifies that key. A Sentry DSN of
//     https://<key>@<host>/v1/o11y/<org> makes the SDK POST to exactly these paths:
//     the SDK appends its own fixed /api/<project>/envelope/ suffix to the DSN path.
//     So the /api/ segment here is the Sentry wire contract — a foreign protocol we
//     receive, like /.well-known — and the route literal is the public path verbatim,
//     with nothing rewriting it in between.
//
//   - READ (Hanzo IAM authz, org-scoped): the Issues list/detail/update the console
//     Errors tab consumes at /v1/o11y/errortracking/issues[/{id}].
//
// The three READ routes are ALSO typed ops at the module's mount seam
// (sentryerrors.go, on the /v1/o11y group), which is what carries them into the
// document, the SDK, the CLI and the agent surface. That is a second DISPATCH, never
// a second implementation: the ops answer by handing the call to THIS router, so the
// handlers below stay the one place the reads are performed. The two INGEST routes
// stay ONLY here — OpenAccess, DSN-authenticated, opaque Sentry-envelope body — a
// deliberate escape hatch out of the document.
func (provider *provider) addErrorTrackingRoutes(router routing.Router) {
	h := provider.errorTrackingHandler

	routes := []struct {
		method string
		path   string
		fn     http.HandlerFunc
		def    handler.OpenAPIDef
	}{
		{http.MethodPost, "/v1/o11y/api/{project_id}/envelope/", provider.authzMiddleware.OpenAccess(h.EnvelopeIngest), handler.OpenAPIDef{
			ID: "IngestErrorEnvelope", Tags: []string{"errortracking"}, Summary: "Ingest a Sentry envelope",
			Description:         "Sentry-envelope-compatible ingest. Authenticated by the DSN public key (X-Sentry-Auth or ?sentry_key), not a Hanzo session.",
			RequestContentType:  "application/x-sentry-envelope",
			ResponseContentType: "application/json", SuccessStatusCode: http.StatusOK,
			ErrorStatusCodes: []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusServiceUnavailable},
			SecuritySchemes:  []handler.OpenAPISecurityScheme{},
		}},
		{http.MethodPost, "/v1/o11y/api/{project_id}/store/", provider.authzMiddleware.OpenAccess(h.StoreIngest), handler.OpenAPIDef{
			ID: "IngestErrorStore", Tags: []string{"errortracking"}, Summary: "Ingest a legacy Sentry store event",
			Description:         "Legacy single-event Sentry ingest. Authenticated by the DSN public key.",
			RequestContentType:  "application/json",
			ResponseContentType: "application/json", SuccessStatusCode: http.StatusOK,
			ErrorStatusCodes: []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusServiceUnavailable},
			SecuritySchemes:  []handler.OpenAPISecurityScheme{},
		}},
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
