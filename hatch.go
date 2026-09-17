package o11y

// The routes that cannot be typed ops, and the net/http seam one is reached
// through. A hatch hands the request to the runtime's handler for its own
// address, unbuffered, so a stream stays a stream.

import (
	"net/http"

	"github.com/zap-proto/zip"
)

// route registers one hatch at its full public address. See mountHatches for
// why each of them cannot be a typed op.
func route(app *zip.App, method, path string) {
	app.Raw(method, path, hatch(method, zip.Template(path)))
}

// bridge is this package's ONE net/http adapter: every route that answers with a
// handler the HOST installed is registered through here. (A typed op does not
// come this way — relay drives the same handler with a recorder, from inside the
// op, and never touches the router.)
//
// What it adapts is net/http on the far side by construction: a runtime resolves
// to an http.Handler ([Runtime]), and a health handler is three
// http.HandlerFuncs (factory.Handler). Neither is a value this package could ask
// for natively — both are handed in by the host — so the request has to be
// rendered as an http.Request once, here, to reach either.
//
// pick runs per request because both callers choose late: a hatch reads the
// runtime SetRuntime installed, a probe the handler SetHealth did, and either can
// change while the process serves. Only the CHOICE is late — the adapter itself
// is built once, at registration. Building it per request is what this replaces:
// probe called zip.AdaptNetHTTP inside its own handler, so every liveness poll
// allocated an adapter and wrote zip's terminal marker into a process-wide map
// again, to answer a request whose shape has not changed since boot.
func bridge(pick func() http.Handler) zip.Handler {
	return zip.AdaptNetHTTP(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pick().ServeHTTP(w, r)
	}))
}

// answer is the runtime's handler for one address, or the refusal that says
// which of the two reasons it has none. The refusal is net/http's own shape —
// text/plain and nosniff — because it is written on the net/http side of the
// bridge, and health_test.go's table pins it that way.
func answer(method, template string) http.Handler {
	h, err := at(method, template)
	if err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
		})
	}
	return h
}

// hatch serves ONE address by handing the request straight to the runtime's
// handler for it — no recorder, no decode, no re-encode.
//
// That pass-through is the point. Every route registered this way is one whose
// answer cannot be named: an unbounded log tail, a long poll, a chunked export, a
// 303 whose whole content is a Location, a Sentry envelope frame. relay buffers a
// complete answer before it can decode one, so a typed op would hang on the first
// tail and would return a progress report only after the query it reports on had
// finished. Here the runtime's handler writes to the caller's own
// ResponseWriter, so a stream stays a stream and a header stays a header.
func hatch(method, template string) zip.Handler {
	return bridge(func() http.Handler { return answer(method, template) })
}
