package o11yapiserver

import (
	"net/http"

	"github.com/hanzoai/o11y/pkg/http/handler"
	"github.com/hanzoai/o11y/pkg/http/routing"
	"github.com/hanzoai/o11y/pkg/types"
	"github.com/hanzoai/o11y/pkg/types/errortrackingtypes"
	"github.com/hanzoai/o11y/pkg/types/sentrytypes"
)

// The ingest {project} path var is constrained to a UUID — {project:guid} — so a
// project segment is a UUID and nothing else, and a garbage segment is a 404 from the
// router rather than a body handed to the DSN verifier. The constraint is the router's
// own `guid` — a UUID parse, not a hand-written character class that has to be read to
// know what it accepts.

// addSentryRoutes registers TWO families, and they answer on two roots because they
// answer two different callers:
//
//   - The FACE, /v1/sentinel — Hanzo Sentry, the Sentry-parity product read by a
//     signed-in person: projects, issues, discover, events, logs, traces, stats. Hanzo
//     IAM authz, every one scoped to the caller's org from the validated claims.
//   - The DOOR, POST /v1/event/{project}/envelope/ and /store/ — a keyed beacon. It is
//     the address a minted DSN spells (implsentry's mintDSN), so the address a client
//     is told and the address this router opens are one string. OpenAccess (no IAM):
//     a Sentry SDK presents a DSN key, not a Hanzo session; the handler verifies that
//     key against the project's rotation watermark.
//
// ONE WIRE, ONE PUBLIC ADDRESS. The door used to sit under the face's root as well,
// which put one wire at two addresses and put an unauthenticated write inside a
// principal-gated subtree — so the face's gate had to carry an exemption shaped like
// the ingest paths, and the exemption is what anything under that root had to be
// checked against. Off the face there is nothing to exempt.
//
// Both are literal paths on the SAME router the o11y read plane uses, so no
// /v1/o11y→/api rewrite applies (see createPublicServer: the path that arrives is the
// path that matches).
//
// FIVE of the read routes — discover, logs, traces, traces/{id}, stats — are ALSO
// declared as typed ops at the module's mount seam (telemetry.go in the repo root),
// which is what carries them into the composed document, the SDK, the CLI and the
// agent surface. That is a second DISPATCH, never a second implementation: the ops
// answer by handing the call to this router, so the handlers below stay the one
// place the reads are performed. Both are needed — the ops serve the composed
// binary, this router serves the standalone process, which has no native router to
// register an op on — so deleting either half drops one of the two deployments.
//
// The OTHER ten reads/writes of this face — projects (list/create/get/delete/
// rotate-key), issues (list/get/update/events) and one event — are ALSO typed ops
// at that seam (sentryerrors.go), the same second DISPATCH into this router. The two
// INGEST routes below are named at that seam as HATCHES rather than ops (mount.go's
// mountHatches): OpenAccess, DSN-authenticated, carrying an opaque Sentry-envelope
// body a typed relay would corrupt, so they are named but not typed and stay out of
// the document by design.
func (provider *provider) addSentryRoutes(router routing.Router) {
	h := provider.sentryHandler

	// The FACE.
	staticRoutes := []struct {
		method string
		path   string
		fn     http.HandlerFunc
		def    handler.OpenAPIDef
	}{
		{http.MethodGet, "/v1/sentinel/projects", provider.authzMiddleware.ViewAccess(h.ListProjects), handler.OpenAPIDef{
			ID: "SentryListProjects", Tags: []string{"sentry"}, Summary: "List Sentry projects",
			Response: new(sentrytypes.GettableProjects), ResponseContentType: "application/json",
			SuccessStatusCode: http.StatusOK, SecuritySchemes: newSecuritySchemes(types.RoleViewer),
		}},
		{http.MethodPost, "/v1/sentinel/projects", provider.authzMiddleware.EditAccess(h.CreateProject), handler.OpenAPIDef{
			ID: "SentryCreateProject", Tags: []string{"sentry"}, Summary: "Create a Sentry project",
			Request: new(sentrytypes.PostableProject), RequestContentType: "application/json",
			Response: new(sentrytypes.GettableProject), ResponseContentType: "application/json",
			SuccessStatusCode: http.StatusOK, ErrorStatusCodes: []int{http.StatusBadRequest},
			SecuritySchemes: newSecuritySchemes(types.RoleEditor),
		}},
		{http.MethodGet, "/v1/sentinel/projects/{id}", provider.authzMiddleware.ViewAccess(h.GetProject), handler.OpenAPIDef{
			ID: "SentryGetProject", Tags: []string{"sentry"}, Summary: "Get a Sentry project",
			Response: new(sentrytypes.GettableProject), ResponseContentType: "application/json",
			SuccessStatusCode: http.StatusOK, ErrorStatusCodes: []int{http.StatusNotFound},
			SecuritySchemes: newSecuritySchemes(types.RoleViewer),
		}},
		{http.MethodDelete, "/v1/sentinel/projects/{id}", provider.authzMiddleware.EditAccess(h.DeleteProject), handler.OpenAPIDef{
			ID: "SentryDeleteProject", Tags: []string{"sentry"}, Summary: "Delete a Sentry project",
			SuccessStatusCode: http.StatusNoContent, ErrorStatusCodes: []int{http.StatusNotFound},
			SecuritySchemes: newSecuritySchemes(types.RoleEditor),
		}},
		{http.MethodPost, "/v1/sentinel/projects/{id}/keys/rotate", provider.authzMiddleware.EditAccess(h.RotateProjectKey), handler.OpenAPIDef{
			ID: "SentryRotateProjectKey", Tags: []string{"sentry"}, Summary: "Rotate a project's DSN key",
			Response: new(sentrytypes.GettableProject), ResponseContentType: "application/json",
			SuccessStatusCode: http.StatusOK, ErrorStatusCodes: []int{http.StatusNotFound},
			SecuritySchemes: newSecuritySchemes(types.RoleEditor),
		}},
		{http.MethodGet, "/v1/sentinel/issues", provider.authzMiddleware.ViewAccess(h.ListIssues), handler.OpenAPIDef{
			ID: "SentryListIssues", Tags: []string{"sentry"}, Summary: "List error issues",
			RequestQuery: new(errortrackingtypes.IssuesQuery), Response: new(errortrackingtypes.GettableIssues),
			ResponseContentType: "application/json", SuccessStatusCode: http.StatusOK,
			SecuritySchemes: newSecuritySchemes(types.RoleViewer),
		}},
		{http.MethodGet, "/v1/sentinel/issues/{id}", provider.authzMiddleware.ViewAccess(h.GetIssue), handler.OpenAPIDef{
			ID: "SentryGetIssue", Tags: []string{"sentry"}, Summary: "Get an error issue",
			Response: new(errortrackingtypes.GettableIssue), ResponseContentType: "application/json",
			SuccessStatusCode: http.StatusOK, ErrorStatusCodes: []int{http.StatusNotFound},
			SecuritySchemes: newSecuritySchemes(types.RoleViewer),
		}},
		{http.MethodPut, "/v1/sentinel/issues/{id}", provider.authzMiddleware.EditAccess(h.UpdateIssue), handler.OpenAPIDef{
			ID: "SentryUpdateIssue", Tags: []string{"sentry"}, Summary: "Update an issue's lifecycle",
			Request: new(errortrackingtypes.UpdateIssue), RequestContentType: "application/json",
			Response: new(errortrackingtypes.Issue), ResponseContentType: "application/json",
			SuccessStatusCode: http.StatusOK, ErrorStatusCodes: []int{http.StatusBadRequest, http.StatusNotFound},
			SecuritySchemes: newSecuritySchemes(types.RoleEditor),
		}},
		{http.MethodGet, "/v1/sentinel/issues/{id}/events", provider.authzMiddleware.ViewAccess(h.IssueEvents), handler.OpenAPIDef{
			ID: "SentryIssueEvents", Tags: []string{"sentry"}, Summary: "List an issue's occurrences",
			ResponseContentType: "application/json", SuccessStatusCode: http.StatusOK,
			SecuritySchemes: newSecuritySchemes(types.RoleViewer),
		}},
		{http.MethodPost, "/v1/sentinel/discover", provider.authzMiddleware.ViewAccess(h.Discover), handler.OpenAPIDef{
			ID: "SentryDiscover", Tags: []string{"sentry"}, Summary: "Query the events plane",
			Request: new(sentrytypes.DiscoverRequest), RequestContentType: "application/json",
			Response: new(sentrytypes.DiscoverResult), ResponseContentType: "application/json",
			SuccessStatusCode: http.StatusOK, ErrorStatusCodes: []int{http.StatusBadRequest},
			SecuritySchemes: newSecuritySchemes(types.RoleViewer),
		}},
		{http.MethodGet, "/v1/sentinel/events/{id}", provider.authzMiddleware.ViewAccess(h.GetEvent), handler.OpenAPIDef{
			ID: "SentryGetEvent", Tags: []string{"sentry"}, Summary: "Get one error event",
			Response: new(sentrytypes.Event), ResponseContentType: "application/json",
			SuccessStatusCode: http.StatusOK, ErrorStatusCodes: []int{http.StatusNotFound},
			SecuritySchemes: newSecuritySchemes(types.RoleViewer),
		}},
		{http.MethodGet, "/v1/sentinel/logs", provider.authzMiddleware.ViewAccess(h.ListLogs), handler.OpenAPIDef{
			ID: "SentryListLogs", Tags: []string{"sentry"}, Summary: "List error-event logs",
			ResponseContentType: "application/json", SuccessStatusCode: http.StatusOK,
			ErrorStatusCodes: []int{http.StatusBadRequest}, SecuritySchemes: newSecuritySchemes(types.RoleViewer),
		}},
		{http.MethodGet, "/v1/sentinel/traces", provider.authzMiddleware.ViewAccess(h.ListTraces), handler.OpenAPIDef{
			ID: "SentryListTraces", Tags: []string{"sentry"}, Summary: "List error-correlated traces",
			ResponseContentType: "application/json", SuccessStatusCode: http.StatusOK,
			ErrorStatusCodes: []int{http.StatusBadRequest}, SecuritySchemes: newSecuritySchemes(types.RoleViewer),
		}},
		{http.MethodGet, "/v1/sentinel/traces/{id}", provider.authzMiddleware.ViewAccess(h.GetTrace), handler.OpenAPIDef{
			ID: "SentryGetTrace", Tags: []string{"sentry"}, Summary: "Get a trace waterfall",
			ResponseContentType: "application/json", SuccessStatusCode: http.StatusOK,
			ErrorStatusCodes: []int{http.StatusBadRequest, http.StatusNotFound}, SecuritySchemes: newSecuritySchemes(types.RoleViewer),
		}},
		{http.MethodGet, "/v1/sentinel/stats", provider.authzMiddleware.ViewAccess(h.Stats), handler.OpenAPIDef{
			ID: "SentryStats", Tags: []string{"sentry"}, Summary: "Event-rate timeseries",
			ResponseContentType: "application/json", SuccessStatusCode: http.StatusOK,
			ErrorStatusCodes: []int{http.StatusBadRequest}, SecuritySchemes: newSecuritySchemes(types.RoleViewer),
		}},
	}
	for _, rt := range staticRoutes {
		router.Handle(rt.method, rt.path, handler.New(rt.fn, rt.def))
	}

	// INGEST — the door, on its own root, UUID-constrained project segment.
	ingestRoutes := []struct {
		path string
		fn   http.HandlerFunc
		def  handler.OpenAPIDef
	}{
		{"/v1/event/{project:guid}/envelope/", provider.authzMiddleware.OpenAccess(h.EnvelopeIngest), handler.OpenAPIDef{
			ID: "SentryIngestEnvelope", Tags: []string{"sentry"}, Summary: "Ingest a Sentry envelope",
			Description:         "Sentry-envelope-compatible ingest. Authenticated by the DSN public key (X-Sentry-Auth or ?sentry_key), not a Hanzo session.",
			RequestContentType:  "application/x-sentry-envelope",
			ResponseContentType: "application/json", SuccessStatusCode: http.StatusOK,
			ErrorStatusCodes: []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusServiceUnavailable},
			SecuritySchemes:  []handler.OpenAPISecurityScheme{},
		}},
		{"/v1/event/{project:guid}/store/", provider.authzMiddleware.OpenAccess(h.StoreIngest), handler.OpenAPIDef{
			ID: "SentryIngestStore", Tags: []string{"sentry"}, Summary: "Ingest a legacy Sentry store event",
			Description:         "Legacy single-event Sentry ingest. Authenticated by the DSN public key.",
			RequestContentType:  "application/json",
			ResponseContentType: "application/json", SuccessStatusCode: http.StatusOK,
			ErrorStatusCodes: []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusServiceUnavailable},
			SecuritySchemes:  []handler.OpenAPISecurityScheme{},
		}},
	}
	for _, rt := range ingestRoutes {
		router.Post(rt.path, handler.New(rt.fn, rt.def))
	}
}
