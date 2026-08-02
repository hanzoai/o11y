package middleware

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"

	"github.com/hanzoai/o11y/pkg/http/handler"
	"github.com/hanzoai/o11y/pkg/types/coretypes"
)

// Resource resolves a route's declared ResourceDefs and stashes the result in
// the request context for authz and audit to read.
type Resource struct {
	logger *slog.Logger
}

func NewResource(logger *slog.Logger) *Resource {
	return &Resource{logger: logger.With(slog.String("pkg", pkgname))}
}

// For builds the middleware for a route with these declared resources.
//
// The defs are a fact about the ROUTE, so they arrive from the router that
// registered it. They used to be recovered per request — CurrentRoute(req) back
// to the route, GetHandler back to the handler, a type assertion back to the
// declaration — three hops to re-derive at request time what was already known
// once at boot. A route with nothing declared now gets no middleware at all
// rather than one that checks and returns.
func (middleware *Resource) For(defs []handler.ResourceDef) func(http.Handler) http.Handler {
	if len(defs) == 0 {
		return func(next http.Handler) http.Handler { return next }
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			// Buffer the body once so extractors can read it and the handler still sees a fresh reader.
			var body []byte
			if req.Body != nil {
				body, _ = io.ReadAll(req.Body)
				req.Body = io.NopCloser(bytes.NewReader(body))
			}

			resolved := handler.ResolveRequest(defs, coretypes.ExtractorContext{
				Request:     req,
				RequestBody: body,
			})

			ctx := coretypes.NewContextWithResolvedResources(req.Context(), resolved)
			next.ServeHTTP(rw, req.WithContext(ctx))
		})
	}
}
