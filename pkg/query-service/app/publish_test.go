package app

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zap-proto/zip"
)

// What publish is FOR, verified the only way this defect was ever visible: by
// the BODY of a request to the router, not by a status code.
//
// The conversion shipped 353 typed operations that no binary linked. Every proof
// it had was taken inside a test process that imported the table directly, which
// is exactly the thing that cannot fail: an uncalled package-level func is legal
// Go, so the package built, the route arithmetic added up, and the server the
// image runs published nothing. Status codes could not see it either — /version
// answers 200 with or without the table, because the service's own registration
// answers it.
//
// So this test drives the SAME function the server calls, over the SAME router
// type it serves, and reads the document out of it.

// documentOf fetches the OpenAPI document off a router the way a caller does.
func documentOf(t *testing.T, app *zip.App) map[string]any {
	t.Helper()
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, zip.SpecPath, nil))
	if err != nil {
		t.Fatalf("GET %s: %v", zip.SpecPath, err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s status=%d, want 200 — the document answers on no port", zip.SpecPath, resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Fatalf("GET %s content-type=%q, want application/json", zip.SpecPath, ct)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	doc := map[string]any{}
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("the document is not JSON: %v — body: %.120s", err, body)
	}
	return doc
}

// operations counts the method+path pairs the document publishes.
func operations(t *testing.T, doc map[string]any) int {
	t.Helper()
	paths, ok := doc["paths"].(map[string]any)
	if !ok {
		t.Fatalf("document paths has type %T", doc["paths"])
	}
	n := 0
	for path, raw := range paths {
		item, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("document path %s has type %T", path, raw)
		}
		for method := range item {
			switch method {
			case "get", "post", "put", "patch", "delete":
				n++
			}
		}
	}
	return n
}

// mcpTools counts the tools the router offers, read the way an MCP client reads
// them: a tools/list over the served MCP endpoint.
//
// It used to be len(app.MCPTools()) — an in-process accessor on the app the test
// had just built, which is the shape of proof this file exists to reject. The
// declaration lives on an app of its own now (see publish), so that accessor
// answers 0 on the serving app whether or not the tool surface is reachable, and
// a count taken off the declaration object instead would prove only that the
// object knows its own ops. Asking the ROUTER is what proves a caller can reach
// them.
func mcpTools(t *testing.T, app *zip.App) int {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/mcp",
		strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("POST /mcp: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /mcp status=%d, want 200 — the tool surface answers on no port", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var out struct {
		Result struct {
			Tools []map[string]any `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("tools/list is not JSON-RPC: %v — body: %.120s", err, body)
	}
	return len(out.Result.Tools)
}

// THE PAYOFF, on the router the binary serves. 353 is the same count
// routes_test.go pins on the table itself; asserting it HERE is what makes the
// two the same 353 rather than two numbers that happen to agree.
func TestPublishServesTheDocument(t *testing.T) {
	app := zip.New(zip.Config{DisableStartupMessage: true})
	if err := publish(app); err != nil {
		t.Fatalf("publish: %v", err)
	}

	if got := operations(t, documentOf(t, app)); got != 353 {
		t.Fatalf("the served document publishes %d operations, want 353", got)
	}
	if got := mcpTools(t, app); got != 353 {
		t.Fatalf("the served router offers %d MCP tools, want 353", got)
	}
}

// The three public roots this service answers on — two faces and the ingest
// endpoint. Named here so the assertion and the message cannot disagree about what
// "o11y's surface" means.
const (
	o11yRootPath     = "/v1/o11y"
	sentinelRootPath = "/v1/o11y/sentinel"
	eventRootPath    = "/v1/event"
)

// Every operation the document publishes is o11y's own surface. A host that
// mounts this table is publishing these paths under its own name, so a path that
// is not ours would be this service claiming someone else's address.
func TestPublishedDocumentIsO11ysSurface(t *testing.T) {
	app := zip.New(zip.Config{DisableStartupMessage: true})
	if err := publish(app); err != nil {
		t.Fatalf("publish: %v", err)
	}

	paths, ok := documentOf(t, app)["paths"].(map[string]any)
	if !ok {
		t.Fatal("document has no paths")
	}
	for path := range paths {
		// Prefix, not a fixed slice: the roots are different lengths, so an index
		// is a second place the root's spelling lives and it goes stale the moment
		// one of them is renamed.
		if !strings.HasPrefix(path, o11yRootPath) && !strings.HasPrefix(path, sentinelRootPath) &&
			!strings.HasPrefix(path, eventRootPath) {
			t.Errorf("the document publishes %s, which is not o11y's surface", path)
		}
	}
}

// THE FIVE CONTROL PATHS, BYTE FOR BYTE.
//
// publish's claim is that routing to the declaration app is "the same decision it
// would make on its own listener". That is a claim about bytes, so it is checked
// as one: every probe below is asked of the serving router AND of a declaration
// app of its own, and the two answers must be identical — status, Content-Type,
// the headers named here, and the body.
//
// The literal columns are what keeps the comparison from being circular. Two
// routers that both stopped answering would agree with each other; a status, a
// content type and, where the answer is short enough to write down, the bytes
// themselves are read off the ROW rather than off the other router.
type answer struct {
	method string
	path   string
	body   string

	status int
	ctype  string
	// allow is the Allow header, set only where the router refuses the method.
	allow string
	// bytes is the whole body, for the answers short enough to state. The long
	// ones — the document, the tool list, the plugin declaration — are pinned by
	// the comparison with the declaration app instead.
	bytes string
}

var answers = []answer{
	{method: http.MethodGet, path: zip.SpecPath, status: 200, ctype: "application/json"},
	{method: http.MethodGet, path: zip.DocsPath, status: 200, ctype: "text/html; charset=utf-8"},
	// Fiber matches with a trailing slash and answers HEAD off the GET route, so
	// both reach the same handler as /docs. Registered once, answering three ways.
	{method: http.MethodGet, path: zip.DocsPath + "/", status: 200, ctype: "text/html; charset=utf-8"},
	{method: http.MethodHead, path: zip.DocsPath, status: 200, ctype: "text/html; charset=utf-8", bytes: ""},
	{method: http.MethodGet, path: zip.PluginPath, status: 200, ctype: "application/json; charset=utf-8"},
	{
		method: http.MethodPost, path: mcpPath, body: `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`,
		status: 200, ctype: "application/json; charset=utf-8",
	},
	{
		method: http.MethodPost, path: mcpPath, body: `{"jsonrpc":"2.0","id":2,"method":"initialize"}`,
		status: 200, ctype: "application/json; charset=utf-8",
		bytes: `{"id":2,"jsonrpc":"2.0","result":{"capabilities":{"tools":{"listChanged":false}},` +
			`"protocolVersion":"2025-06-18","serverInfo":{"name":"o11y","version":""}}}`,
	},
	// The call plane names the op in the path and answers in ZAP, refusal
	// included — application/json is the boundary encoding, and this plane is not
	// the boundary.
	{
		method: http.MethodPost, path: zip.CallPath + "nope", body: `{}`,
		status: 404, ctype: "application/zap",
		bytes: "ZAP\x00\x01\x00\x00\x00\x10\x00\x00\x008\x00\x00\x00\x94\x01\x00\x00" +
			"\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\b\x00\x00\x00\x10\x00\x00\x00unknown op: nope",
	},

	// A MISS IS THE ROUTER'S OWN, and it does not look like net/http's. There is
	// no path cleaning in front of these: a dot segment, a bare "." segment and a
	// doubled slash are all matched literally and all miss, where an http.ServeMux
	// would have answered 301 to the cleaned form before matching anything. The
	// body is JSON, not "404 page not found", and no X-Content-Type-Options rides
	// with it (see nosniffIsNotOurs).
	{method: http.MethodPost, path: zip.CallPath, body: `{}`, status: 404, ctype: "application/json; charset=utf-8", bytes: missBody},
	{method: http.MethodGet, path: "/docs/../docs", status: 404, ctype: "application/json; charset=utf-8", bytes: missBody},
	{method: http.MethodGet, path: "//docs", status: 404, ctype: "application/json; charset=utf-8", bytes: missBody},
	{method: http.MethodGet, path: "/./docs", status: 404, ctype: "application/json; charset=utf-8", bytes: missBody},
	{method: http.MethodGet, path: zip.SpecPath + "/extra", status: 404, ctype: "application/json; charset=utf-8", bytes: missBody},
	{method: http.MethodGet, path: "/missing", status: 404, ctype: "application/json; charset=utf-8", bytes: missBody},
	{
		method: http.MethodPost, path: zip.DocsPath,
		status: 405, ctype: "application/json; charset=utf-8", allow: "GET, HEAD",
		bytes: `{"status":405,"error":"Method Not Allowed"}`,
	},
}

const missBody = `{"status":404,"error":"Not Found"}`

// ask puts one probe to a router and returns the whole answer.
func (a answer) ask(t *testing.T, app *zip.App) (*http.Response, []byte) {
	t.Helper()
	var body io.Reader
	if a.body != "" {
		body = strings.NewReader(a.body)
	}
	req := httptest.NewRequest(a.method, a.path, body)
	if a.body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("%s %s: %v", a.method, a.path, err)
	}
	read, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("%s %s: read body: %v", a.method, a.path, err)
	}
	return resp, read
}

func TestPublishServesTheDeclarationVerbatim(t *testing.T) {
	app := zip.New(zip.Config{DisableStartupMessage: true})
	if err := publish(app); err != nil {
		t.Fatalf("publish: %v", err)
	}
	// A declaration of its own, to answer the same probes on its own router.
	own, err := declaration()
	if err != nil {
		t.Fatalf("declaration: %v", err)
	}

	for _, a := range answers {
		at := a.method + " " + a.path
		resp, got := a.ask(t, app)

		if resp.StatusCode != a.status {
			t.Errorf("%s status=%d, want %d", at, resp.StatusCode, a.status)
		}
		if ct := resp.Header.Get("Content-Type"); ct != a.ctype {
			t.Errorf("%s content-type=%q, want %q", at, ct, a.ctype)
		}
		if allow := resp.Header.Get("Allow"); allow != a.allow {
			t.Errorf("%s allow=%q, want %q", at, allow, a.allow)
		}
		// net/http writes this beside every http.Error; a zip answer carries it
		// only when a handler sets it, and none of these does.
		if nosniff := resp.Header.Get("X-Content-Type-Options"); nosniff != "" {
			t.Errorf("%s x-content-type-options=%q, want none", at, nosniff)
		}
		if a.bytes != "" || a.method == http.MethodHead {
			if string(got) != a.bytes {
				t.Errorf("%s body=%q, want %q", at, got, a.bytes)
			}
		}

		mine, want := a.ask(t, own)
		if resp.StatusCode != mine.StatusCode {
			t.Errorf("%s status=%d on the serving router, %d on the declaration's own",
				at, resp.StatusCode, mine.StatusCode)
		}
		if ct, mct := resp.Header.Get("Content-Type"), mine.Header.Get("Content-Type"); ct != mct {
			t.Errorf("%s content-type=%q on the serving router, %q on the declaration's own", at, ct, mct)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s answers %d bytes, the declaration's own router %d — they must be the same bytes",
				at, len(got), len(want))
		}
	}
}

// WHY THE DECLARATION IS DISPATCHED TO AND NOT COMPOSED IN.
//
// app.Use(d) would read the declaration's entries as if they had been written on
// the serving app, which is exactly what must not happen: both name the same 367
// addresses, and Build refuses a program where two definitions claim one address.
// The refusal names the address and both claimants, so the reason this is a
// dispatch is measurable rather than asserted in a comment.
func TestTheDeclarationCannotBeComposedIn(t *testing.T) {
	app := zip.New(zip.Config{AppName: "o11y", DisableStartupMessage: true})
	const taken = "/v1/o11y/logs"
	app.Get(taken, func(c *zip.Ctx) error { return c.NoContent(http.StatusOK) })

	d, err := declaration()
	if err != nil {
		t.Fatalf("declaration: %v", err)
	}
	app.Use(d)

	err = app.Build()
	if err == nil {
		t.Fatal("composing the declaration into the serving app built cleanly; one address now has two definitions")
	}
	if !strings.Contains(err.Error(), "GET "+taken) {
		t.Fatalf("Build refused with %q, which does not name %s", err, taken)
	}
}

// The declaration does NOT stand in front of the implementation. publish is
// called after the service's own registration precisely so the handler that has
// always answered a path still answers it — a typed op in front would put every
// answer through a buffered relay round-trip, and an unbounded stream (livetail,
// the chunked export) does not survive being buffered.
//
// Registration order is invisible to the compiler and to every test that mounts
// only one of the two, so it is pinned here: an earlier registration wins, and
// publish's routes are the later ones.
func TestServiceRoutesOutrankTheDeclaration(t *testing.T) {
	app := zip.New(zip.Config{DisableStartupMessage: true})

	const path = "/v1/o11y/logs"
	served := false
	app.Get(path, zip.AdaptNetHTTP(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		served = true
		w.WriteHeader(http.StatusOK)
	})))

	if err := publish(app); err != nil {
		t.Fatalf("publish: %v", err)
	}

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, path, nil))
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	if !served {
		t.Fatalf("the declaration answered %s; the service's own handler must", path)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s status=%d, want 200", path, resp.StatusCode)
	}
}
