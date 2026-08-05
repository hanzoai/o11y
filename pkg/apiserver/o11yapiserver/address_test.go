package o11yapiserver

import (
	"net/http"
	"testing"

	"github.com/hanzoai/o11y"
	"github.com/hanzoai/o11y/pkg/http/routing"
	"github.com/zap-proto/zip"
)

// BY NAME, NOT BY COUNT.
//
// routes_test.go counts this registrar's routes and the module's declaration
// counts its own, and 233 + 134 = 367 on both sides. Two sets of 367 can be 367
// DIFFERENT addresses and every count in the repo stays green — which is not a
// hypothetical: three of these routes bound {traceID} where the declaration
// named {traceId}, and nothing could see it, because a router matches by
// POSITION and a parameter's name is invisible to it. It only became visible
// when something wanted to look an address UP by name, which is what the seam
// between the two now does (routing.Table.Handler).
//
// So this compares the sets. Every address this registrar serves must be one the
// module declares, and must resolve through the table by that same name — the
// two properties the seam depends on, checked against the real registration
// rather than against a list.
//
// Equality follows without a second test: this file and its sibling in
// pkg/query-service/app each prove registered ⊆ declared, their counts are 233
// and 134, the declaration's is 367, and a duplicate registration panics at boot
// (routing.Handle). 233 + 134 distinct addresses inside a set of 367 is the whole
// set.
func TestEveryRegisteredAddressIsDeclaredAndResolves(t *testing.T) {
	app := zip.New(zip.Config{DisableStartupMessage: true})
	router := routing.New(app.Group(""), nil)
	censusProvider().AddToRouter(router)
	table := router.Table()

	for _, route := range table.Routes() {
		if !declared(t)[route.Method+" "+route.Path] {
			t.Errorf("%s %s is served here and declared nowhere — a caller reaching it "+
				"is in no document, and the seam cannot name it", route.Method, route.Path)
		}
		if table.Handler(route.Method, route.Path) == nil {
			t.Errorf("%s %s registered but does not resolve by name", route.Method, route.Path)
		}
	}
}

// declared is every address the module's own declaration of this service names,
// in the PUBLIC spelling both sides state an address in.
func declared(t *testing.T) map[string]bool {
	t.Helper()
	d := zip.New(zip.Config{DisableStartupMessage: true})
	if err := o11y.Mount(d); err != nil {
		t.Fatalf("Mount: %v", err)
	}
	out := map[string]bool{}
	for _, r := range d.Fiber().GetRoutes(true) {
		if r.Method == http.MethodHead || r.Method == http.MethodOptions {
			continue
		}
		out[r.Method+" "+zip.Template(r.Path)] = true
	}
	return out
}
