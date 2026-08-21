package o11y_test

// THE REACHABILITY CENSUS.
//
// routes_test.go counts what Mount REGISTERS. This file measures what a caller
// can actually REACH, which is a different number and was never the same one.
//
// The gap is the whole defect class. A typed op reaches its handler, the handler
// calls a relay, and the relay hands the runtime an http.Request. If that
// request is not server-SHAPED the runtime cannot route it, and the caller gets
// 404 from a route that is unmistakably registered. The route census is green
// the entire time — it never leaves the router.
//
// This bit once, was fixed in ONE of the five relay bodies, and the four copies
// kept the defect. 94 of 366 routes still answered 404 after the fix that was
// believed to have closed it. The reason a copy could survive the fix is that
// nothing drove the whole surface at a runtime that ROUTES: the existing
// regression exercised /version and /health, both served by the one body that
// had been repaired.
//
// So this census drives EVERY registered route through the production embed
// shape — adaptor.FiberApp over a runtime that routes, which is exactly what
// Server.PublicHandler() returns and what the host installs — against a runtime
// that serves every one of those paths. Any 404 is a request that never reached
// the runtime, i.e. a lost door. There is no way to add a sixth relay body with
// this defect and keep this test green.

import (
	"net/http"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/hanzoai/o11y"
	fiber "github.com/zap-proto/fiber/v3"
	"github.com/zap-proto/fiber/v3/middleware/adaptor"
	"github.com/zap-proto/zip"
)

// concrete turns a registered route pattern into a path a caller can request,
// substituting a real value for every parameter. Fiber spells parameters
// ":name" (with an optional "?"), and a "+"/"*" segment is a wildcard.
func concrete(pattern string) string {
	parts := strings.Split(pattern, "/")
	for i, seg := range parts {
		switch {
		case seg == "":
		case seg[0] == ':':
			// A UUID satisfies the ops that parse their id as one, and is a
			// perfectly ordinary string for the ops that do not.
			parts[i] = "6ba7b810-9dad-11d1-80b4-00c04fd430c8"
		case seg == "*" || seg == "+":
			parts[i] = "x"
		}
	}
	return strings.Join(parts, "/")
}

// routingRuntime installs a runtime that SERVES every pattern the mount
// registered, wrapped the way the embedding host wraps it. Only a request that
// carries a real request-target can match; a request whose target was erased
// falls through to fiber's own not-found, which is the signal this file reads.
//
// Deliberately no catch-all: a catch-all would match the "/" that an erased
// target normalizes to and would answer 200 for a request that reached nothing,
// which is precisely the false pass this test exists to refuse.
// It answers 204 with no body on purpose. This file measures ONE thing — does
// the request arrive — and a stand-in payload would drag in a second: every
// typed Out would be decoded from it and re-marshalled on the way back, so an
// unrelated marshalling defect in any one of 366 Out types would land here
// wearing a reachability failure's clothes. 204 keeps the concerns orthogonal:
// relay treats it as success, the decode of an empty body fails, and the caller
// gets 500 — which still proves arrival, because a request that never arrived
// gets 404 instead.
func routingRuntime(t *testing.T, patterns map[string]bool) (seen *[]string) {
	t.Helper()
	var targets []string
	var mu sync.Mutex
	rt := zip.New(zip.Config{DisableStartupMessage: true})
	answer := func(c fiber.Ctx) error {
		mu.Lock()
		targets = append(targets, string(c.Request().RequestURI()))
		mu.Unlock()
		return c.SendStatus(http.StatusNoContent)
	}
	for route := range patterns {
		method, path, _ := strings.Cut(route, " ")
		rt.Fiber().Add([]string{method}, path, answer)
	}
	o11y.SetRuntime(o11y.Whole(adaptor.FiberApp(rt.Fiber())))
	t.Cleanup(func() { o11y.SetRuntime(nil) })
	return &targets
}

// TestEveryRouteReachesTheRuntime is the payoff measurement: registered and
// reachable are the same set.
//
// The assertion is on 404 alone. Every other status proves the request ARRIVED
// — a 400 from input validation, a 500 from decoding a stand-in body into a
// typed Out, a 401 from the gate — and this file is only ever about whether the
// door opens onto anything.
func TestEveryRouteReachesTheRuntime(t *testing.T) {
	app := mounted(t)
	patterns := registered(t, app)
	seen := routingRuntime(t, patterns)

	// The three native probes answer themselves and never delegate; they are the
	// only registered routes for which reaching no runtime is correct.
	probes := map[string]bool{
		"GET /v1/o11y/livez": true, "GET /v1/o11y/healthz": true, "GET /v1/o11y/readyz": true,
	}

	var lost, erased []string
	for route := range patterns {
		method, pattern, _ := strings.Cut(route, " ")
		if probes[route] {
			continue
		}
		path := concrete(pattern)
		before := len(*seen)
		status, _ := call(t, app, member(method, path, strings.NewReader("{}")))
		if status == http.StatusNotFound {
			lost = append(lost, method+" "+pattern)
			continue
		}
		// The defect's fingerprint, checked directly rather than through a status:
		// an erased request-target normalizes to "/" at the adaptor, so the runtime
		// is asked for a path the caller never named.
		for _, got := range (*seen)[before:] {
			if target, _, _ := strings.Cut(got, "?"); target != path {
				erased = append(erased, method+" "+pattern+" -> runtime saw "+got)
			}
		}
	}

	if len(lost) > 0 {
		sort.Strings(lost)
		t.Errorf("%d of %d registered routes answer 404 — they never reached the runtime.\n"+
			"A relay that does not set the request-target erases the path, so the "+
			"runtime routes on \"/\" and matches nothing:\n  %s",
			len(lost), len(patterns), strings.Join(lost, "\n  "))
	}
	if len(erased) > 0 {
		sort.Strings(erased)
		t.Errorf("%d routes reached the runtime at a path the caller did not name:\n  %s",
			len(erased), strings.Join(erased, "\n  "))
	}
}

// TestEveryRelayIsTheOneRelay is the structural half. The census above catches a
// broken relay by its symptom; this catches a SECOND relay by its existence.
//
// relay.go's doc has claimed "one function, one place: every typed op reaches
// the o11y runtime through relay and through nothing else" since the collapse
// that wrote it. That was aspirational, not true — four more bodies sat in the
// op files, each a copy of relay carrying its own drift. A copy is how the
// request-target fix reached one call path and missed a hundred and eight.
//
// So the invariant is spelled where it can fail: exactly one function in this
// package may hand a request to the runtime handler.
func TestEveryRelayIsTheOneRelay(t *testing.T) {
	callers := handlerCallers(t)
	sort.Strings(callers)
	if len(callers) != 1 || callers[0] != "relay" {
		t.Errorf("functions that serve a request to the runtime handler: %v\n"+
			"want exactly [relay] — a second body is a second place for the "+
			"request-target, the identity headers and the refusal reader to drift.",
			callers)
	}
}
