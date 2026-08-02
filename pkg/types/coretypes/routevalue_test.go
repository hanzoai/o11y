package coretypes_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hanzoai/o11y/pkg/http/routing"
	"github.com/hanzoai/o11y/pkg/types/coretypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const ruleRoute = "/v1/o11y/llm_pricing_rules/{id}"

// serve drives one request through a REAL router registered at template, and
// hands the handler's request back. Testing the reader against the writer alone
// would prove only that the two halves of this file agree with each other; what
// has to hold is that the reader reads what the ROUTER put there.
func serve(t *testing.T, template, method, target string) *http.Request {
	t.Helper()

	var got *http.Request
	router := routing.Serve(method, template, http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
		got = req
	}))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(method, target, http.NoBody))
	require.Equal(t, http.StatusOK, rec.Code, "the route did not match — the test is not exercising the reader")
	require.NotNil(t, got)
	return got
}

func TestParamReadsTheSegmentTheRouterMatched(t *testing.T) {
	req := serve(t, ruleRoute, http.MethodGet, "/v1/o11y/llm_pricing_rules/0198c4e2-4f6a-7c1a-8a2b-000000000001")

	assert.Equal(t, "0198c4e2-4f6a-7c1a-8a2b-000000000001", coretypes.Param(req, "id"))
	// A name the route does not declare is absent, not an error — every caller
	// already treats "" as "not given".
	assert.Empty(t, coretypes.Param(req, "project_id"))
}

// A path segment and a query value are different kinds of value carried by
// different halves of the URL, and coretypes.Param answers for exactly one of them.
func TestParamIsNotAQueryRead(t *testing.T) {
	req := serve(t, ruleRoute, http.MethodGet, "/v1/o11y/llm_pricing_rules/7?limit=25")

	assert.Equal(t, "7", coretypes.Param(req, "id"))
	assert.Empty(t, coretypes.Param(req, "limit"), "limit is a query value; coretypes.Param must not answer for it")
	assert.Equal(t, "25", req.URL.Query().Get("limit"))
}

func TestParamOnARequestThatNeverMetARouter(t *testing.T) {
	assert.Empty(t, coretypes.Param(httptest.NewRequest(http.MethodGet, "/v1/o11y/llm_pricing_rules/7", http.NoBody), "id"))
	assert.Empty(t, coretypes.Param(nil, "id"))
}

// The injector must produce what the router produces, or a handler test passes
// against a shape the serving path never delivers. Both the segment AND the
// template: an audit record keyed off an injected request has to say the same
// thing as one keyed off a served request.
func TestSetRouteMatchesTheRouter(t *testing.T) {
	routed := serve(t, ruleRoute, http.MethodGet, "/v1/o11y/llm_pricing_rules/7")
	injected := coretypes.SetRoute(httptest.NewRequest(http.MethodGet, "/v1/o11y/llm_pricing_rules/7", http.NoBody), ruleRoute)

	assert.Equal(t, coretypes.Param(routed, "id"), coretypes.Param(injected, "id"))
	assert.Equal(t, coretypes.RoutePath(routed), coretypes.RoutePath(injected))
}

func TestRoutePathIsTheTemplateNotTheRequestPath(t *testing.T) {
	req := serve(t, ruleRoute, http.MethodGet, "/v1/o11y/llm_pricing_rules/7")

	assert.Equal(t, ruleRoute, coretypes.RoutePath(req))
	assert.Equal(t, "/v1/o11y/llm_pricing_rules/7", req.URL.Path)
}

// Unmatched answers "" rather than panicking, so the audit record falls back to
// the request path instead of taking the process down on a 404.
func TestRoutePathWithoutAMatchedRoute(t *testing.T) {
	assert.Empty(t, coretypes.RoutePath(httptest.NewRequest(http.MethodGet, "/v1/o11y/llm_pricing_rules/7", http.NoBody)))
	assert.Empty(t, coretypes.RoutePath(nil))
}

// The authz extractor names a resource by its path segment, and it must do so
// through the same read — one router, one param model, one answer.
func TestPathParamExtractorReadsThroughParam(t *testing.T) {
	req := serve(t, ruleRoute, http.MethodGet, "/v1/o11y/llm_pricing_rules/7")

	id, ran := coretypes.PathParam("id").RunFor(coretypes.PhaseRequest, coretypes.ExtractorContext{Request: req})
	assert.True(t, ran)
	assert.Equal(t, "7", id)

	id, ran = coretypes.PathParam("id").RunFor(coretypes.PhaseRequest, coretypes.ExtractorContext{})
	assert.True(t, ran)
	assert.Empty(t, id, "no request means no segment, not a panic")
}
