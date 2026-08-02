package coretypes

import (
	"net/http"

	"github.com/gorilla/mux"
)

// THE ROUTE-VALUE SEAM. One reader, one writer, one template read — and this
// file is the ONLY place in the service that knows which router filled them.
//
// A path segment and a query parameter are two different kinds of value and
// this package keeps them apart, because conflating them is how a route quietly
// starts reading the wrong half of the URL:
//
//	/rules/{id}   is a PATH segment  -> coretypes.Param(req, "id")   -> zip c.Param("id")
//	?limit=25     is a QUERY value   -> req.URL.Query().Get("limit") -> zip c.Query("limit")
//
// Param is for handlers that still take (http.ResponseWriter, *http.Request). A
// handler that has a *zip.Ctx already has c.Param and must use it directly —
// this seam is not a second way to do the same thing, it is the SAME read for
// the callers that have no Ctx to read it from.
//
// Why the indirection exists at all: before it, 140 handler bodies each named
// gorilla/mux to read a path segment, so the router was braided into every
// piece of business logic that addresses a resource by id. Which router matched
// the route is not a fact a handler should know — the VALUE it needs is the
// segment. With the read named once, converting the tree to zip routes changes
// the three function bodies below and nothing else; before it, that conversion
// touched every handler in the service and would have failed SILENTLY (an
// unmatched mux.Vars returns "", not an error, so an id would arrive empty and
// the handler would 404 or, worse, act on the zero value).

// Param returns one PATH segment of the matched route: for the route
// /v1/o11y/llm_pricing_rules/{id} and the request /v1/o11y/llm_pricing_rules/7,
// Param(req, "id") is "7". An unmatched name, or a request that reached the
// handler without going through the router, yields "" — the same absence the
// callers have always handled.
func Param(req *http.Request, name string) string {
	if req == nil {
		return ""
	}
	return mux.Vars(req)[name]
}

// SetParams injects path segments into a request so a handler can be exercised
// without standing up a router. It is the WRITER half of the read above, and it
// belongs next to it: a test that injects through one router's private API
// while the handler reads through another's passes for the wrong reason.
//
// Tests only. Nothing in the serving path calls it — the router fills these.
func SetParams(req *http.Request, params map[string]string) *http.Request {
	return mux.SetURLVars(req, params)
}

// RoutePath returns the matched route's path TEMPLATE — the registered literal
// "/v1/o11y/llm_pricing_rules/{id}", not the request's "/v1/o11y/llm_pricing_rules/7".
// That distinction is the whole point: a metric or an audit record keyed on the
// concrete path has one series per id and is useless.
//
// It answers "" when there is no matched route or the route has no template, so
// a caller keeps its own fallback rather than being handed a guess.
func RoutePath(req *http.Request) string {
	if req == nil {
		return ""
	}
	route := mux.CurrentRoute(req)
	if route == nil {
		return ""
	}
	path, err := route.GetPathTemplate()
	if err != nil {
		return ""
	}
	return path
}
