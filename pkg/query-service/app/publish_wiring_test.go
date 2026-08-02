package app

// THE LINK, NOT THE FUNCTION.
//
// publish_test.go proves what publish DOES: mount it on a bare zip.App and 353
// operations and 353 MCP tools appear. Every assertion it makes calls publish
// directly — which is precisely the shape of the original defect. 353 typed ops
// once shipped in files nothing called; the package built, the route arithmetic
// added up, and the running server published nothing. A test that calls the
// function itself cannot see that, because the thing that was missing was the
// CALL.
//
// Deleting `publish(app)` from createPublicServer leaves the entire o11y suite
// green — measured, not assumed. The only evidence that the link exists was
// `go tool nm` on ./cmd/community, run by hand, in no CI.
//
// So state the invariant where it can fail. createPublicServer is the
// composition root for this service's surface, and two facts about it are load-
// bearing:
//
//   - it calls publish, or the document, the MCP tool list and the call plane are
//     all empty in the process that serves them;
//   - it calls publish BEFORE the console catch-all app.All("/*", …), because
//     the router matches in registration order and a terminal route registered
//     first would swallow every published path.
//
// Constructing a real Server to observe this would need the whole module graph —
// stores, telemetry, licensing — so the wiring is read from the source, which is
// where the defect lives and the only place it is visible at all.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"strings"
	"testing"
)

// compositionRoot returns the ordered list of calls createPublicServer makes, as
// "receiver.Selector" / "function" names.
func compositionRoot(t *testing.T) []string {
	t.Helper()
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi fs.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatalf("parse package: %v", err)
	}

	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Name.Name != "createPublicServer" || fn.Body == nil {
					continue
				}
				var calls []string
				ast.Inspect(fn.Body, func(n ast.Node) bool {
					call, ok := n.(*ast.CallExpr)
					if !ok {
						return true
					}
					switch fun := call.Fun.(type) {
					case *ast.Ident:
						calls = append(calls, fun.Name)
					case *ast.SelectorExpr:
						if id, ok := fun.X.(*ast.Ident); ok {
							calls = append(calls, id.Name+"."+fun.Sel.Name)
						} else {
							calls = append(calls, fun.Sel.Name)
						}
					}
					return true
				})
				return calls
			}
		}
	}
	t.Fatal("createPublicServer not found — this test is measuring the wrong thing")
	return nil
}

func TestCreatePublicServerPublishesTheSurface(t *testing.T) {
	calls := compositionRoot(t)

	publishAt, catchAllAt := -1, -1
	for i, name := range calls {
		switch name {
		case "publish":
			if publishAt < 0 {
				publishAt = i
			}
		case "app.All":
			if catchAllAt < 0 {
				catchAllAt = i
			}
		}
	}

	if publishAt < 0 {
		t.Fatal("createPublicServer does not call publish — the running server registers no typed " +
			"operations, publishes no OpenAPI document and exposes no MCP tools, and every test " +
			"in this package still passes because they all call publish themselves")
	}
	if catchAllAt < 0 {
		t.Fatal("createPublicServer no longer registers the console catch-all app.All(\"/*\", …); " +
			"this test's ordering assertion is measuring something that moved")
	}
	if publishAt > catchAllAt {
		t.Fatalf("publish is called at position %d, after the console catch-all at %d — the router "+
			"matches in registration order, so the SPA shell would answer every published path",
			publishAt, catchAllAt)
	}
}
