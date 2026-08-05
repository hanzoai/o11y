package o11y

// THE SEAM, AND WHY IT IS A NAME AND NOT A HANDLER.
//
// This table DECLARES 367 addresses. The o11y runtime — telemetry stores, rule
// manager, opamp, the query engine — IMPLEMENTS the same 367. The declaration is
// not a second implementation (that is the whole discipline: an op names the
// wire, it does not re-serve it), so every op has to reach the implementation,
// and the shape of that reach is the design decision.
//
// It used to be ONE HANDLER FOR EVERYTHING. A host installed the runtime's whole
// public surface as a single http.Handler and every op spoke HTTP to it: build a
// request at the op's path, hand it over, let the runtime's router MATCH that
// path and dispatch. Which means the declaration knew exactly which door it
// wanted, threw the address away, and paid a router to find it again — inside the
// same process, through an httptest recorder, across a net/http↔fasthttp bridge,
// 353 times over.
//
// The cost was never only the hop. Two things could not be said:
//
//   - "This address is not implemented." A declared address the runtime does not
//     serve came back as the runtime's own 404, indistinguishable from a caller's
//     typo, discovered by a customer. It is now a nil at this seam, which a test
//     can read at boot. The two sides already drift: three addresses this table
//     declares as {traceId} the runtime registered as {traceID}, and nothing
//     could see it, because a path is matched by position and a parameter's NAME
//     is invisible to a router.
//   - "The runtime is right here." The standalone server IS the runtime, and it
//     could not install itself as that one handler without an op relaying into the
//     router it was already being served by. So it installed nothing — and every
//     op on its /mcp tool surface and its by-name call plane answered 503, in the
//     binary that has all 367 handlers in memory. The declaration was published
//     by a process that could not answer it.
//
// A [Runtime] answers BY NAME. It is what a route table already is — a map from
// address to handler — read as one. The standalone server hands over its own
// table; the composed cloud binary hands over the embedded runtime's; a host
// whose runtime is in another process hands over [Whole], because for a remote
// every address really does resolve to the same door.

import (
	"net/http"
	"sync"

	"github.com/zap-proto/zip"
)

// Runtime is the implementation this declaration names: the handler that answers
// ONE address.
//
// method and path are an address in the PUBLIC spelling — "GET",
// "/v1/o11y/traces/{traceId}" — which is the string this table declares, the
// string the document publishes and the string the runtime's own registration
// records. One spelling on both sides, or they are not the same address.
//
// A nil answer means the runtime does not serve that address, and the op says so
// rather than guessing.
type Runtime interface {
	Handler(method, path string) http.Handler
}

// Func is a Runtime that resolves with a function, the way http.HandlerFunc is a
// Handler that serves with one. Runtime has a single method, so a host that wants
// to DECORATE the reach — hanzoai/cloud puts its own principal gate around each
// answer — otherwise has to declare a struct and a method to say one expression.
//
// It is what keeps the decoration out of here. Resolution is o11y's concern and
// admission is the host's; composing them wants nothing more than a function that
// calls one and wraps the other.
type Func func(method, path string) http.Handler

// Handler resolves through f.
func (f Func) Handler(method, path string) http.Handler { return f(method, path) }

// Whole is a Runtime whose every address is the same handler — the honest shape
// for a runtime that is somewhere ELSE. A reverse proxy has one door and the
// request's own path is what selects the route on the other side, so there is
// nothing to resolve here and pretending otherwise would be a lie with a map in
// it.
func Whole(h http.Handler) Runtime { return whole{h} }

type whole struct{ h http.Handler }

func (w whole) Handler(string, string) http.Handler { return w.h }

var (
	rmu     sync.RWMutex
	runtime Runtime
)

// SetRuntime installs the implementation every op in this table reaches.
//
// The standalone server installs its OWN route table, which is the case the
// previous seam could not express: there the declaration and the implementation
// are the same process, and naming the address is what lets an op call the
// handler directly instead of relaying into the router that is already serving
// it. The composed cloud binary installs the embedded runtime's table, or
// [Whole] over a proxy when it runs the runtime as a separate Deployment.
//
// Safe for concurrent use; pass nil to unset.
func SetRuntime(rt Runtime) {
	rmu.Lock()
	runtime = rt
	rmu.Unlock()
}

// answers is the handler for one address, or nil when no runtime is installed or
// the installed one does not serve it.
func answers(method, path string) http.Handler {
	rmu.RLock()
	rt := runtime
	rmu.RUnlock()
	if rt == nil {
		return nil
	}
	return rt.Handler(method, path)
}

// at is the handler for one address, or the refusal that says which of the two
// reasons it has none. The distinction is the whole point of naming addresses:
// "nothing is installed" is a deployment that has not finished booting, and "this
// address is not served" is a declaration that has drifted from its runtime.
func at(method, path string) (http.Handler, error) {
	rmu.RLock()
	rt := runtime
	rmu.RUnlock()
	if rt == nil {
		return nil, zip.Errorf(http.StatusServiceUnavailable, "o11y runtime not installed")
	}
	h := rt.Handler(method, path)
	if h == nil {
		return nil, zip.Errorf(http.StatusServiceUnavailable, "o11y runtime does not serve %s %s", method, path)
	}
	return h, nil
}
