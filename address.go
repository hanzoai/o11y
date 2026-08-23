package o11y

// AN OPERATION'S ADDRESS IS A FACT OF ITS REGISTRATION, NOT AN ARGUMENT OF ITS
// BODY.
//
// Every op in this table is declared at an address and then has to reach the
// runtime that implements that address. Until now it reached it by saying the
// address a second time, by hand, inside the handler:
//
//	opGet(g, "/traces/:traceId", traceSpans)                    // the declaration
//	relay(ctx, "GET", o11yRoot+"/traces/"+in.TraceID, …)        // the body
//
// Two spellings of one value, 366 times over, and the second one is what got
// requested — so the first could stop agreeing with it and never be contradicted.
// It has: three of these addresses declare {traceId} where the runtime registered
// {traceID}. A router matches by POSITION, so a parameter's name is invisible to
// it and the disagreement cost nothing until something wanted to look an address
// up by name. That is exactly what the seam now does.
//
// So the address travels WITH the call. The declaration verbs (claim.go) wrap
// every op in [addressed], which puts the address the op was registered at into
// the context, and relay reads it there. The op's body states no method and no
// path at all, which is the honest end state: which endpoint was called is not
// something the body decides.
//
// The concrete path comes from [zip.Address], the inverse of the binding zip
// already performed — zip put the matched segments INTO the input, so reading
// them back out through the same rule cannot disagree with what arrived. The 119
// hand-built concatenations it replaces each re-derived that by eye.

import (
	"context"

	"github.com/zap-proto/zip"
)

// addr is one operation's address: the method, the public TEMPLATE that names it
// at the seam, and the CONCRETE path this particular call is for.
//
// Two spellings live here on purpose and they are two different values. The
// template is the address's NAME — "/v1/o11y/traces/{traceId}" — the string the
// document publishes and the string the runtime's registration recorded, so it is
// what resolves a handler. The path is where THIS call is going —
// "/v1/o11y/traces/abc-123" — so it is what the request carries.
type addr struct {
	method   string
	template string
	path     string
}

// addrKey names the context slot an op's address rides in. A zero-size unexported
// type: unforgeable from outside this package, so nothing can plant an address an
// op was not registered at.
type addrKey struct{}

// addressed states an op's address once and carries it into the call.
//
// The template is computed at REGISTRATION — once per op for the life of the
// process — because it is a property of the declaration. Only the concrete path
// is per-call, because only it depends on the input.
//
// The full address is the target's own prefix plus the declared path, the same
// composition [claims] reads and the same one zip's registry records, so the
// three cannot name different addresses.
func addressed[In, Out any](on zip.OpTarget, method, path string, fn zip.TypedHandler[In, Out]) zip.TypedHandler[In, Out] {
	pattern := on.OpScope().Prefix + path
	template := zip.Template(pattern)
	return func(ctx context.Context, in *In) (*Out, error) {
		return fn(context.WithValue(ctx, addrKey{}, addr{
			method:   method,
			template: template,
			path:     zip.Address(pattern, in),
		}), in)
	}
}

// addressOf is the address of the op whose body is running, and whether there is
// one. There is not exactly when an op's function is called directly rather than
// through the declaration that registered it — a programming error, and one worth
// naming rather than papering over with a guessed address.
func addressOf(ctx context.Context) (addr, bool) {
	a, ok := ctx.Value(addrKey{}).(addr)
	return a, ok
}
