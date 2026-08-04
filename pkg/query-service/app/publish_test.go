package app

import (
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
// them: a tools/list over the served door.
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

// THE PAYOFF, on the router the binary serves. 350 is the same count
// routes_test.go pins on the table itself; asserting it HERE is what makes the
// two the same 350 rather than two numbers that happen to agree.
//
// It was 353. Three addresses — GET /v1/o11y/logs, GET /v1/o11y/metrics and
// POST /v1/o11y/query_range — are the composing HOST's to declare, because the
// host answers them with a tenant-scoped handler and the table's relay ops did
// not carry an org at all. A document holds one operation per method+path, so
// the table yields the claim; the reason is written out at each removal site.
// The service still SERVES all three — the implementation is registered at
// publish's call site, on the listener, and is untouched by this. What moved is
// only which of the two apps DESCRIBES them.
func TestPublishServesTheDocument(t *testing.T) {
	app := zip.New(zip.Config{DisableStartupMessage: true})
	if err := publish(app); err != nil {
		t.Fatalf("publish: %v", err)
	}

	if got := operations(t, documentOf(t, app)); got != 350 {
		t.Fatalf("the served document publishes %d operations, want 350", got)
	}
	if got := mcpTools(t, app); got != 350 {
		t.Fatalf("the served router offers %d MCP tools, want 350", got)
	}
}

// Every operation the document publishes is o11y's own surface. A host that
// mounts this table is publishing these paths under its own name, so a path that
// is not ours would be this service claiming someone else's door.
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
		if len(path) < 4 || (path[:8] != "/v1/o11y" && path[:10] != "/v1/sentry") {
			t.Errorf("the document publishes %s, which is not o11y's surface", path)
		}
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
