package coretypes

import (
	"context"
	"net/http"
	"strings"
)

// THE ROUTE-VALUE SEAM. One reader, one writer, one template read — and this
// file is the ONLY place in the service that knows anything about how a route's
// values got onto a request.
//
// A path segment and a query parameter are two different kinds of value and
// this package keeps them apart, because conflating them is how a route quietly
// starts reading the wrong half of the URL:
//
//	/rules/{id}   is a PATH segment  -> coretypes.Param(req, "id")
//	?limit=25     is a QUERY value   -> req.URL.Query().Get("limit")
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
// segment. With the read named once, converting the tree to zip routes changed
// the functions below and nothing else; before it, that conversion would have
// touched every handler in the service and would have failed SILENTLY (an
// unmatched lookup returns "", not an error, so an id would arrive empty and
// the handler would 404 or, worse, act on the zero value).

// routeKey names the request-scoped slot the matched route's values live in. A
// zero-size unexported type: unforgeable from outside this package, and the same
// key whether the values were put there by the router or by a test.
type routeKey struct{}

// routeValues is what the router matched: the path TEMPLATE and the segments it
// bound. Kept together because they are one fact — a template without its
// bindings names a route nothing was read from.
type routeValues struct {
	template string
	params   map[string]string
}

// BindRoute states the matched route's values through set, whatever the router's
// own request-scoped setter is. It takes the setter rather than the router's
// context type so this package stays free of the router's transport: the only
// thing it needs is somewhere keyed to put one value.
func BindRoute(set func(key, value any), template string, params map[string]string) {
	set(routeKey{}, routeValues{template: template, params: params})
}

// SetRoute binds a request to the route TEMPLATE it answers, filling the
// segments the template names from the request's own path. It is the WRITER half
// of the read below, and it belongs next to it: a test that injects through one
// mechanism while the handler reads through another passes for the wrong reason.
//
// It is the writer for a request that arrives at a route BY NAME rather than by
// matching — the module's own declaration of this surface calls its handler
// directly, having named the address, so no router runs in front of it (see
// routing.Table.Handler). Deriving the segments positionally is exact there,
// because the caller built the path from that same template.
//
// BindRoute is the writer for a request the ROUTER matched, which knows its
// bindings without deriving them. Two entries, one representation, one read.
func SetRoute(req *http.Request, template string) *http.Request {
	if req == nil {
		return nil
	}
	names := strings.Split(strings.TrimPrefix(template, "/"), "/")
	got := strings.Split(strings.TrimPrefix(req.URL.Path, "/"), "/")
	params := map[string]string{}
	for i, name := range names {
		if i >= len(got) || !strings.HasPrefix(name, "{") || !strings.HasSuffix(name, "}") {
			continue
		}
		params[name[1:len(name)-1]] = got[i]
	}
	return req.WithContext(context.WithValue(req.Context(), routeKey{}, routeValues{template: template, params: params}))
}

// Param returns one PATH segment of the matched route: for the route
// /v1/o11y/llm_pricing_rules/{id} and the request /v1/o11y/llm_pricing_rules/7,
// Param(req, "id") is "7". An unmatched name, or a request that reached the
// handler without going through the router, yields "" — the same absence the
// callers have always handled.
func Param(req *http.Request, name string) string {
	if req == nil {
		return ""
	}
	values, _ := req.Context().Value(routeKey{}).(routeValues)
	return values.params[name]
}

// RoutePath returns the matched route's path TEMPLATE — the registered literal
// "/v1/o11y/llm_pricing_rules/{id}", not the request's "/v1/o11y/llm_pricing_rules/7".
// That distinction is the whole point: a metric or an audit record keyed on the
// concrete path has one series per id and is useless.
//
// It answers "" when there is no matched route, so a caller keeps its own
// fallback rather than being handed a guess.
func RoutePath(req *http.Request) string {
	if req == nil {
		return ""
	}
	values, _ := req.Context().Value(routeKey{}).(routeValues)
	return values.template
}
