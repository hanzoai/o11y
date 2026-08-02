package o11y_test

// handlerCallers is the structural probe behind TestEveryRelayIsTheOneRelay.
//
// It names every function in the package that SYNTHESIZES a request for the
// runtime: one that builds an http.Request and hands it to the runtime
// handler's ServeHTTP. Both halves are required, which is what separates a
// relay from mount.go's handlerAdapter — the adapter forwards the request the
// SERVER already built (whose request-target the server itself populated) and
// so can never carry this defect. A relay builds the request, and a built
// request is only server-shaped if the builder makes it so.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"strings"
	"testing"
)

func handlerCallers(t *testing.T) []string {
	t.Helper()
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi fs.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatalf("parse package: %v", err)
	}

	var found []string
	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil {
					continue
				}
				var builds, serves bool
				ast.Inspect(fn.Body, func(n ast.Node) bool {
					call, ok := n.(*ast.CallExpr)
					if !ok {
						return true
					}
					sel, ok := call.Fun.(*ast.SelectorExpr)
					if !ok {
						return true
					}
					if id, ok := sel.X.(*ast.Ident); ok && id.Name == "http" &&
						strings.HasPrefix(sel.Sel.Name, "NewRequest") {
						builds = true
					}
					if sel.Sel.Name == "ServeHTTP" {
						serves = true
					}
					return true
				})
				if builds && serves {
					found = append(found, fn.Name.Name)
				}
			}
		}
	}
	return found
}
