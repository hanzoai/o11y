package o11y_test

// THE SEAM RESOLVES BY NAME, AND THIS FILE IS WHERE THAT IS TRUE OR NOT.
//
// Every other file installs a runtime that ROUTES — a fiber app, or a
// HandlerFunc that answers whatever it is handed. Both of those pass whether the
// address was looked up or merely requested, so neither can tell the two designs
// apart. The runtime here answers at EXACT addresses and at nothing else: it has
// no router, no catch-all and no fall-through, so a call that arrives is a call
// whose address was resolved.
//
// That is also what the design buys. A request cannot lose its route to a router
// that is never consulted — which is the whole of the defect class the
// reachability census exists for, deleted rather than guarded.

import (
	"bytes"
	"net/http"
	"sort"
	"strings"
	"testing"

	"github.com/hanzoai/o11y"
	fiber "github.com/zap-proto/fiber/v3"
	"github.com/zap-proto/fiber/v3/middleware/adaptor"
	"github.com/zap-proto/zip"
)

// named answers at exact addresses and at nothing else.
type named map[string]http.Handler

func (n named) Handler(method, path string) http.Handler { return n[method+" "+path] }

// TestByNameArrivesWhereMatchingDid IS THE A/B, and it is exact.
//
// Both designs are driven over the same 367 addresses with the same requests.
// The old one hands every call to a single handler with a ROUTER behind it and
// lets the path be matched a second time; the new one looks the address up and
// calls the handler. The two must deliver the same calls to the same paths, or
// the change moved a route.
//
// It is an A/B rather than an arrival count because 60 of these ops refuse their
// own input before the seam is reached — a required field the driver cannot
// invent — and that is true of both designs equally. Comparing them to each other
// measures the thing that changed and nothing else; comparing either to a number
// would pin an unrelated fact about input validation.
func TestByNameArrivesWhereMatchingDid(t *testing.T) {
	app := mounted(t)
	byMatch := drive(t, app, matching)
	byName := drive(t, app, exact)

	var missing, extra, moved []string
	for address, path := range byMatch {
		got, arrived := byName[address]
		switch {
		case !arrived:
			missing = append(missing, address)
		case got != path:
			moved = append(moved, address+": matching asked for "+path+", by-name asked for "+got)
		}
	}
	for address := range byName {
		if _, ok := byMatch[address]; !ok {
			extra = append(extra, address)
		}
	}

	for what, list := range map[string][]string{
		"stopped reaching the runtime":            missing,
		"reached the runtime at a different path": moved,
		"reached the runtime only by name":        extra,
	} {
		if len(list) > 0 {
			sort.Strings(list)
			t.Errorf("%d addresses %s:\n  %s", len(list), what, strings.Join(list, "\n  "))
		}
	}
	if len(byName) == 0 {
		t.Fatal("nothing arrived at all — the driver is measuring nothing")
	}
}

// drive calls every declared address once and reports, per address, the
// request-target the runtime was asked for.
func drive(t *testing.T, app *zip.App, install func(*testing.T, map[string]bool, *string)) map[string]string {
	t.Helper()
	patterns := registered(t, app)

	var asked string
	install(t, patterns, &asked)

	// The three native probes answer themselves and never delegate; reaching no
	// runtime is correct for them under either design.
	probes := map[string]bool{
		"GET /v1/o11y/livez": true, "GET /v1/o11y/healthz": true, "GET /v1/o11y/readyz": true,
	}

	out := map[string]string{}
	for route := range patterns {
		if probes[route] {
			continue
		}
		method, pattern, _ := strings.Cut(route, " ")
		asked = ""
		call(t, app, member(method, concrete(pattern), strings.NewReader("{}")))
		if asked != "" {
			out[method+" "+zip.Template(pattern)] = asked
		}
	}
	return out
}

// matching is the design this change replaces: ONE handler for the whole
// surface, with a router behind it that matches the path a second time.
func matching(t *testing.T, patterns map[string]bool, asked *string) {
	t.Helper()
	rt := zip.New(zip.Config{DisableStartupMessage: true})
	answer := func(c fiber.Ctx) error {
		*asked = strings.Clone(string(c.Request().RequestURI()))
		return c.SendStatus(http.StatusNoContent)
	}
	for route := range patterns {
		method, path, _ := strings.Cut(route, " ")
		rt.Fiber().Add([]string{method}, path, answer)
	}
	o11y.SetRuntime(o11y.Whole(adaptor.FiberApp(rt.Fiber())))
	t.Cleanup(func() { o11y.SetRuntime(nil) })
}

// exact is the design this change installs: a handler per ADDRESS and no router
// anywhere. Nothing here can match, so a call that arrives is a call whose
// address was resolved.
func exact(t *testing.T, patterns map[string]bool, asked *string) {
	t.Helper()
	rt := named{}
	for route := range patterns {
		method, pattern, _ := strings.Cut(route, " ")
		rt[method+" "+zip.Template(pattern)] = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Cloned: an adapted request's path can be a view into the router's own
			// recycled buffer, and a test that keeps the view reads whatever the NEXT
			// request put there.
			*asked = strings.Clone(r.URL.RequestURI())
			w.WriteHeader(http.StatusNoContent)
		})
	}
	o11y.SetRuntime(rt)
	t.Cleanup(func() { o11y.SetRuntime(nil) })
}

// A DECLARED ADDRESS THE RUNTIME DOES NOT SERVE IS SAYABLE NOW.
//
// It used to be the runtime's own 404 — indistinguishable from a caller's typo,
// and discovered by a customer. Naming the address makes the two reasons two
// answers, and the refusal names the address, so the drift is readable rather
// than inferable.
func TestUnservedAddressSaysSo(t *testing.T) {
	app := mounted(t)
	o11y.SetRuntime(named{})
	t.Cleanup(func() { o11y.SetRuntime(nil) })

	status, body := call(t, app, member(http.MethodGet, "/v1/o11y/version", nil))
	if status != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", status)
	}
	if !strings.Contains(string(body), "GET /v1/o11y/version") {
		t.Errorf("refusal = %s, want it to name the address it could not resolve", body)
	}
}

// No runtime at all is the OTHER reason, and it reads differently: a deployment
// that has not finished booting is not a declaration that has drifted.
func TestNoRuntimeSaysSo(t *testing.T) {
	app := mounted(t)
	o11y.SetRuntime(nil)

	status, body := call(t, app, member(http.MethodGet, "/v1/o11y/version", nil))
	if status != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", status)
	}
	if !strings.Contains(string(body), "not installed") {
		t.Errorf("refusal = %s, want it to say no runtime is installed", body)
	}
}

// THE PAYOFF, AND IT IS THE CASE THE OLD SEAM COULD NOT EXPRESS.
//
// A typed op is reachable three ways: over its route, as an MCP tool, and by
// NAME on the call plane. The last two do not go through a router at all — there
// is no path to match, only an operation id — so they have always depended on the
// seam and nothing else.
//
// In the standalone server that seam was empty. The runtime could not install
// itself as one handler without an op relaying into the router it was already
// being served by, so it installed nothing, and every one of those 353 tools and
// commands answered 503 in the binary that holds all 367 handlers. Naming the
// address is what makes the same process able to answer its own declaration:
// SetRuntime takes the route table, and a call by name resolves to the handler
// registered at the op's address.
func TestCallByNameReachesTheRuntime(t *testing.T) {
	app := mounted(t)
	if err := app.Build(); err != nil {
		t.Fatalf("Build: %v", err)
	}

	const address = "GET /v1/o11y/version"
	reached := false
	o11y.SetRuntime(named{address: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		_, _ = w.Write([]byte(`{"version":"v1.5.49","ee":"N","setupCompleted":true}`))
	})})
	t.Cleanup(func() { o11y.SetRuntime(nil) })

	var name string
	for _, route := range app.Declaration().Routes {
		if route.Method+" "+route.Pattern == address {
			name = route.Op
		}
	}
	if name == "" {
		t.Fatalf("%s declares no operation name, so it is on no call plane", address)
	}

	// The call plane is service-to-service and its body is ZAP; this op needs no
	// input, so an empty one is the whole request.
	status, body := call(t, app, member(http.MethodPost, zip.CallPath+name, nil))
	if status != http.StatusOK {
		t.Fatalf("POST %s%s = %d %s, want 200", zip.CallPath, name, status, body)
	}
	if !reached {
		t.Fatal("the call plane answered without reaching the runtime")
	}
	if !bytes.Contains(body, []byte("v1.5.49")) {
		t.Errorf("call plane answered %q, want it to carry the runtime's own version", body)
	}

	// And the contrast, which is what this binary used to answer for all 353:
	// with no runtime installed the same call is a 503, not an answer.
	o11y.SetRuntime(nil)
	if status, _ := call(t, app, member(http.MethodPost, zip.CallPath+name, nil)); status != http.StatusServiceUnavailable {
		t.Fatalf("with no runtime, POST %s%s = %d, want 503", zip.CallPath, name, status)
	}
}
