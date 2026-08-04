package o11y

// A HOST MAY ALREADY SERVE AN ADDRESS THIS TABLE DECLARES, and when it does it
// has to be able to say so.
//
// This module owns /v1/o11y as its own service address (HIP-0106) and, since the
// conversion, declares every route under it by name — there is no wildcard left
// for a host's own route to win against. That is the property that makes the
// table honest, and it is also what turns a host's overlapping route from a
// silent shadow into a refusal to compose: zip validates that one address is
// declared once, so a host declaring GET /v1/o11y/metrics alongside this table
// does not get precedence, it gets a panic.
//
// In-order shadowing used to make the overlap invisible. It was never a decision,
// only an artefact of registration order, and the two handlers it hid could drift
// apart forever without anyone being told. Making the host NAME the addresses it
// takes is the same fact written down where it can be read.
//
// So the seam is a statement of ownership, not a filter: [Claimed] says "the host
// serves these, this table does not declare them". Everything else is declared
// exactly as before. An address named here that this table never declares is
// inert rather than an error — the host is describing what IT serves, and a
// table that stopped declaring an address should not turn into a boot failure in
// every host that had claimed it.

import (
	"net/http"

	"github.com/zap-proto/zip"
)

// Option configures one [Mount].
type Option func(*conf)

// conf is one Mount's settings.
type conf struct {
	// claimed holds "METHOD /path" for every address the host serves itself.
	claimed map[string]bool
}

// Claimed names addresses the HOST serves itself, which this table therefore does
// not declare. Each is a method and a path, spelled the way zip spells an address
// in its own diagnostics — "GET /v1/o11y/metrics" — so the string that names the
// conflict is the string that resolves it.
//
// The path is the FULL public path, not one relative to the mount root: a host
// reads it off the route it registered, and an address is the same value on both
// sides of this seam or it is not the same address.
//
// A claim is the host taking responsibility for that address. Nothing about the
// runtime changes — the operation is still there, still reachable on the o11y
// runtime's own router — but this table stops publishing it, so the host's
// handler is the one and only answer at that address in the host's process.
func Claimed(addrs ...string) Option {
	return func(c *conf) {
		if c.claimed == nil {
			c.claimed = make(map[string]bool, len(addrs))
		}
		for _, a := range addrs {
			c.claimed[a] = true
		}
	}
}

// mounting is the settings of the Mount currently running. Mount is a
// composition-root call — once per process, before it serves — and every
// declaration below happens inside it, so the set is written once and read
// during that one call. It is nil at every other moment, which is what makes an
// unclaimed mount cost nothing.
var mounting *conf

// claims reports whether the host has taken the address this declaration would
// register at.
//
// The full address is the target's own prefix plus the declared path, so this
// reads the same whether an op is declared on a rooted group (the usual case,
// where path is "/metrics") or straight on the app with a full path. The prefix
// comes from [zip.OpTarget], the interface every declaration already goes
// through, so nothing here needs to know which group it was handed.
func claims(on zip.OpTarget, method, path string) bool {
	if mounting == nil || len(mounting.claimed) == 0 {
		return false
	}
	return mounting.claimed[method+" "+on.OpScope().Prefix+path]
}

// The five declaration verbs, each the zip one plus the claim check. Every typed
// op in this package goes through these rather than through zip directly, which
// is what makes a claim mean the same thing at all 358 declarations instead of
// at the handful someone remembered.
//
// They exist because the check needs the ADDRESS, and an address is the target's
// prefix plus the path — known only at the declaration, not at the target. zip
// resolves a target once, without a path, so a target cannot refuse one address
// and accept another; the verb can.

func opGet[In, Out any](on zip.OpTarget, path string, fn zip.TypedHandler[In, Out], opts ...zip.OpOption) {
	if claims(on, http.MethodGet, path) {
		return
	}
	zip.Get(on, path, fn, opts...)
}

func opPost[In, Out any](on zip.OpTarget, path string, fn zip.TypedHandler[In, Out], opts ...zip.OpOption) {
	if claims(on, http.MethodPost, path) {
		return
	}
	zip.Post(on, path, fn, opts...)
}

func opPut[In, Out any](on zip.OpTarget, path string, fn zip.TypedHandler[In, Out], opts ...zip.OpOption) {
	if claims(on, http.MethodPut, path) {
		return
	}
	zip.Put(on, path, fn, opts...)
}

func opPatch[In, Out any](on zip.OpTarget, path string, fn zip.TypedHandler[In, Out], opts ...zip.OpOption) {
	if claims(on, http.MethodPatch, path) {
		return
	}
	zip.Patch(on, path, fn, opts...)
}

func opDelete[In, Out any](on zip.OpTarget, path string, fn zip.TypedHandler[In, Out], opts ...zip.OpOption) {
	if claims(on, http.MethodDelete, path) {
		return
	}
	zip.Delete(on, path, fn, opts...)
}

// route declares one of the eleven escape hatches — a raw route rather than a
// typed op (see mountHatches) — and honours a claim the same way. A hatch is
// still an address, so a host has to be able to take one.
func route(app *zip.App, method, path string, h zip.Handler) {
	if claims(app, method, path) {
		return
	}
	switch method {
	case http.MethodGet:
		app.Get(path, h)
	case http.MethodPost:
		app.Post(path, h)
	default:
		panic("o11y: route: unsupported method " + method)
	}
}
