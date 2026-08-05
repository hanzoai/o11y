// Package routing is the ONE place a path becomes a handler in this service.
//
// Every route the o11y runtime serves — the 205 declared operations of
// pkg/apiserver/o11yapiserver and the 134 of pkg/query-service/app — is
// registered through a Router, and a Router puts them on zip. There is no second
// router, no subrouter tree with its own matching rules, and no framework left
// whose escape hatches a handler could reach for: the only thing a registration
// site can say is method, path, handler.
//
// WHY IT IS A TYPE AND NOT JUST zip.Router. A registration carries three facts
// that zip has no slot for, and each of them used to be RECOVERED at request
// time by asking the router which route it had just matched:
//
//  1. The path SEGMENTS the route binds (/rules/{id} → id). The handlers are
//     net/http handlers, so they cannot reach c.Param; the values are bound onto
//     the request instead, and coretypes.Param is the one read of them.
//  2. The path TEMPLATE itself. Audit records and metrics key on the template —
//     keyed on the concrete path they are one series per id and useless.
//  3. The route's declared RESOURCES, which the authz/audit resolver needs.
//
// All three are known HERE, at registration, and the router is where they are
// stated. The tree this replaces looked them up dynamically —
// mux.CurrentRoute(req).GetHandler() type-asserted back to handler.Handler in
// two separate packages — which is a static fact laundered through a lookup.
// Stating them once at registration deletes both lookups and the interface
// assertion behind them.
//
// PATH SPELLING. Routes are declared in the brace spelling the public contract
// is written in — "/v1/o11y/rules/{id}" — and this package respells them for the
// router ("/v1/o11y/rules/:id"). One spelling in the source, one translation, in
// one function.
package routing

import (
	"net/http"
	"strings"

	"github.com/hanzoai/o11y/pkg/http/handler"
	"github.com/hanzoai/o11y/pkg/types/coretypes"
	"github.com/zap-proto/fiber/v3"
	"github.com/zap-proto/fiber/v3/middleware/adaptor"
	"github.com/zap-proto/zip"
)

// Chain is the middleware a route is served through, built for the route's
// declared resources.
//
// It takes the defs because the Resource middleware needs them and the ROUTER is
// where that fact is known. Handing them in at registration is what lets the
// middleware stay one uniform net/http chain while the one route-dependent
// member of it stops asking the router what it just matched.
//
// A nil Chain serves handlers bare — what a test that exercises one handler
// wants, and what the o11y runtime never uses.
type Chain func(defs []handler.ResourceDef) func(http.Handler) http.Handler

// Route is one registration: the method, the PUBLIC template in brace spelling,
// and the handler that answers it.
//
// Handler is the WHOLE answer — the chain composed around the leaf, not the bare
// body underneath it. A route's handler is what happens when you knock on that
// door, and the audit record, the timeout and the resource resolution are part of
// what happens. Recording the bare one would make the census describe a route
// nobody can reach.
type Route struct {
	Method  string
	Path    string
	Handler http.Handler
}

// Table is every route registered through a Router and the Groups made from it.
// It exists because a route table that cannot be counted is a route table that
// can go dark: this package's whole surface is registration, so the census of
// what registration did is the only evidence the routes are there.
type Table struct {
	routes []Route
	// at is the same registrations keyed by address, so a caller that KNOWS the
	// address does not have to re-derive it by matching. See [Table.Handler].
	at map[string]http.Handler
}

// Routes returns the registrations in the order they were made — which is the
// order the router matches them in, so the slice IS the precedence.
func (t *Table) Routes() []Route {
	if t == nil {
		return nil
	}
	return t.routes
}

// Handler is the answer at one ADDRESS, reached by name.
//
// THE TABLE IS THE SEAM. This service's surface is stated twice — once here, as
// the implementation, and once in the module's own declaration of it
// (github.com/hanzoai/o11y's typed ops), which names the same 367 addresses with
// an input, an output and their prose. The declaration has always had to REACH
// the implementation, and until now it did so by speaking HTTP to the whole
// service: it built a request, handed it to an http.Handler that was the entire
// router, and that router matched the path a second time to find the handler the
// declaration had already named. An in-process round trip to answer a question
// the caller had the answer to.
//
// A registration is a map from address to handler. Reading it as one deletes the
// second match — and, more than that, it deletes the possibility of the two sides
// disagreeing silently: a declared address that no registration answers is a nil
// here, at the seam, instead of a 404 from a router the declaration never
// intended to consult.
//
// path is the PUBLIC template in brace spelling — "/v1/o11y/traces/{traceId}",
// the same string [Route.Path] carries and the same string the document
// publishes. That is the one spelling the two sides agree on: the router's
// (":traceId", ":project<guid>") carries matching rules that are the router's own
// business and that the declaration does not state.
//
// A nil answer means no registration claims that address.
func (t *Table) Handler(method, path string) http.Handler {
	if t == nil {
		return nil
	}
	return t.at[method+" "+path]
}

// Router registers routes on a zip router. Copying one is free and copies share
// the Table: a Group is a Router with a longer prefix, not a separate tree.
type Router struct {
	zip   zip.Router
	chain Chain
	table *Table

	// pfx is the composed prefix, in the SPELLING the public template uses.
	//
	// It used to be read back off the zip router (OpScope().Prefix), which was the
	// right instinct — one source, no drift — and stopped being possible at zip
	// v1.23. A group is now an App INCLUDED at a prefix rather than a router
	// carrying one, and one definition may be included at two prefixes, so the
	// absolute path is a property of the INCLUSION and zip's walk computes it.
	// App.OpScope() leaves Prefix zero for every group, and reading it still
	// COMPILES — the Table would silently have recorded every nested route at its
	// leaf path. The prefix is therefore composed here, at the one call that knows
	// it, by the same join the recorded path uses.
	pfx string
}

// New starts a route tree on r, serving every handler through chain.
func New(r zip.Router, chain Chain) Router {
	return Router{zip: r, chain: chain, table: &Table{}}
}

// Table returns the census of everything registered through this Router,
// including through the Groups made from it.
func (r Router) Table() *Table { return r.table }

// Group returns a Router that registers under prefix. The prefix is a literal
// path segment sequence, exactly as the tree it replaces used it.
func (r Router) Group(prefix string) Router {
	r.zip = r.zip.Group(colonize(prefix))
	r.pfx = join(r.pfx, prefix)
	return r
}

// Handle registers h to answer method at path. Path is the public template in
// brace spelling; the Router respells it for the router and records it verbatim.
//
// A duplicate registration PANICS. Registration runs once, at boot, from a fixed
// list of literals — a second route at the same door is a mistake in the source,
// not a condition a running server can encounter, and the router that used to
// hold these silently let the first one win.
func (r Router) Handle(method, path string, h http.Handler) Router {
	full := template(join(r.prefix(), path))
	if r.table.at == nil {
		r.table.at = map[string]http.Handler{}
	}
	if _, taken := r.table.at[method+" "+full]; taken {
		panic("routing: " + method + " " + full + " is registered twice")
	}

	served := h
	if r.chain != nil {
		served = r.chain(resourceDefs(h))(h)
	}
	// What the census records and what a by-name caller reaches are ONE value.
	// Recording the chain here is also what lets a caller that arrives by name
	// rather than by matching still be served exactly what the router serves —
	// see [Table.Handler] and [addressed].
	answer := addressed{template: full, served: served}
	r.table.routes = append(r.table.routes, Route{Method: method, Path: full, Handler: answer})
	r.table.at[method+" "+full] = answer

	leaf := bind(full, zip.AdaptNetHTTP(served))
	spelled := colonize(path)
	switch method {
	case http.MethodGet:
		r.zip.Get(spelled, leaf)
	case http.MethodPost:
		r.zip.Post(spelled, leaf)
	case http.MethodPut:
		r.zip.Put(spelled, leaf)
	case http.MethodPatch:
		r.zip.Patch(spelled, leaf)
	case http.MethodDelete:
		r.zip.Delete(spelled, leaf)
	default:
		panic("routing: " + method + " " + full + " uses a method this service does not serve")
	}
	return r
}

func (r Router) Get(path string, h http.Handler) Router {
	return r.Handle(http.MethodGet, path, h)
}

func (r Router) Post(path string, h http.Handler) Router {
	return r.Handle(http.MethodPost, path, h)
}

func (r Router) Put(path string, h http.Handler) Router {
	return r.Handle(http.MethodPut, path, h)
}

func (r Router) Patch(path string, h http.Handler) Router {
	return r.Handle(http.MethodPatch, path, h)
}

func (r Router) Delete(path string, h http.Handler) Router {
	return r.Handle(http.MethodDelete, path, h)
}

// prefix is the composed prefix — see the field. Group composes it with the same
// join Handle records with, so a recorded path cannot drift from where the route
// landed.
func (r Router) prefix() string { return r.pfx }

// Serve is a one-route tree, as a net/http handler.
//
// It exists for the tests that exercise a handler ADDRESSED BY PATH — everything
// that reads coretypes.Param, and every handler whose behaviour depends on the
// segment the router bound. Those tests have to drive the real registrar: a test
// that injects path values by hand and then reads them back proves only that the
// injector and the reader agree with each other, which is exactly the agreement
// that breaks when the router changes. Driving this instead means the same
// registration, the same respelling and the same binding the server uses.
//
// It answers as an http.Handler so a test keeps its httptest.ResponseRecorder
// and its assertions unchanged. The request's own context does NOT survive the
// bridge into the router — nothing carries a Go context across a wire — so a
// test that needs claims on the request adds them in the handler it registers,
// where the router has already bound the route.
func Serve(method, template string, h http.Handler) http.Handler {
	app := zip.New(zip.Config{DisableStartupMessage: true})
	New(app.Group(""), nil).Handle(method, template, h)
	return adaptor.FiberApp(app.Fiber())
}

// addressed is one route's whole answer, carrying the address it answers at.
//
// A handler reached BY MATCHING is handed the route's values by the router that
// matched it (see bind). A handler reached BY NAME has no router in front of it,
// and the two members of the chain that need the route — Audit keys its record on
// the template, Resource resolves the route's declared resources — would read an
// empty route. So the address travels WITH the handler: the value that knows the
// template is the value that was registered at it.
//
// The values are derived from the request's own path against that template, which
// is exact because the caller built the path FROM the template ([zip.Address]).
type addressed struct {
	template string
	served   http.Handler
}

func (a addressed) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.served.ServeHTTP(w, coretypes.SetRoute(r, a.template))
}

// bind puts the matched route's values on the request before the net/http chain
// runs. The values ride the request-scoped slot fasthttp already keeps, which is
// what the adapted handler's context resolves through — so coretypes.Param and
// coretypes.RoutePath are the same read for a handler on the router and a
// handler a test drives directly.
func bind(template string, next zip.Handler) zip.Handler {
	names := params(template)
	return func(c *zip.Ctx) error {
		values := make(map[string]string, len(names))
		for _, name := range names {
			values[name] = c.Param(name)
		}
		coretypes.BindRoute(c.Fiber().RequestCtx().SetUserValue, template, values)
		return next(c)
	}
}

// resourceDefs reads a handler's declared resources, or none. The assertion is
// the one this package makes so nothing else has to: a plain http.HandlerFunc
// declares nothing, and that is not an error.
func resourceDefs(h http.Handler) []handler.ResourceDef {
	if declared, ok := h.(handler.Handler); ok {
		return declared.ResourceDefs()
	}
	return nil
}

// A declared segment is "{name}", or "{name:constraint}" when the route only
// accepts some values of it — the two Sentry ingest paths take a UUID project
// and nothing else. These three functions are the whole translation between the
// declared spelling and everything downstream of it, and they agree by
// construction because each reads the same split of the same segment.
//
//	declared        {project:guid}
//	colonize   →    :project<guid>   what the router matches on
//	template   →    {project}        what the document and the census name
//	params     →    project          what the request is bound with

// colonize respells a declared path for the router.
//
// A constraint the router does not know is a PANIC here, because the router's
// own answer to one is to DROP it: an unrecognised name parses to "no
// constraint", and a dropped constraint matches everything. That is the failure
// mode this service can least afford to have be silent — the two Sentry ingest
// routes are wildcards that sit after the static /v1/sentry words, and they are
// safe there only because the constraint refuses anything that is not a UUID.
// Spell it with a pattern the router does not recognise and the wildcard
// silently swallows every /v1/sentry/<word>/envelope/ instead.
func colonize(path string) string {
	return respell(path, func(name, constraint string) string {
		if constraint == "" {
			return ":" + name
		}
		if !known[constraint] {
			panic("routing: " + path + " constrains " + name + " with " + constraint +
				", which the router does not know and would silently drop")
		}
		return ":" + name + "<" + constraint + ">"
	})
}

// known is the router's OWN set of constraint names, read off its exported
// constants rather than copied as strings — a second list would drift from the
// thing it is guarding. Constraints that take data (min(3), regex(...)) are not
// here: nothing declares one, and the day something does, this list is where the
// spelling gets thought about instead of being discovered in production.
var known = map[string]bool{
	fiber.ConstraintInt:   true,
	fiber.ConstraintBool:  true,
	fiber.ConstraintFloat: true,
	fiber.ConstraintAlpha: true,
	fiber.ConstraintGUID:  true,
}

// template is the declared path with its constraints dropped: the PUBLIC
// contract's spelling, which is what a client writes and what the document says.
func template(path string) string {
	return respell(path, func(name, constraint string) string { return "{" + name + "}" })
}

// params names the segments a path binds, in order.
func params(path string) []string {
	var names []string
	respell(path, func(name, constraint string) string {
		names = append(names, name)
		return ""
	})
	return names
}

// respell rewrites every declared segment of path through say.
func respell(path string, say func(name, constraint string) string) string {
	if !strings.Contains(path, "{") {
		return path
	}
	segments := strings.Split(path, "/")
	for i, segment := range segments {
		if !strings.HasPrefix(segment, "{") || !strings.HasSuffix(segment, "}") {
			continue
		}
		name, constraint, _ := strings.Cut(segment[1:len(segment)-1], ":")
		segments[i] = say(name, constraint)
	}
	return strings.Join(segments, "/")
}

// join composes a group's prefix with a leaf path the way the router does. The
// prefix arrives already respelled (it came off the router), so it is put back
// into the declared spelling to keep one spelling in the source.
func join(prefix, path string) string {
	prefix = strings.TrimRight(brace(prefix), "/")
	if prefix == "" {
		return path
	}
	if path == "" {
		return prefix
	}
	if path[0] != '/' {
		path = "/" + path
	}
	return prefix + path
}

// brace is colonize read backwards, for the one value that arrives respelled.
func brace(path string) string {
	if !strings.Contains(path, ":") {
		return path
	}
	segments := strings.Split(path, "/")
	for i, segment := range segments {
		if strings.HasPrefix(segment, ":") {
			name, constraint, found := strings.Cut(strings.TrimSuffix(segment[1:], ">"), "<")
			if found {
				segments[i] = "{" + name + ":" + constraint + "}"
				continue
			}
			segments[i] = "{" + name + "}"
		}
	}
	return strings.Join(segments, "/")
}
