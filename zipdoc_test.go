package o11y_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"strconv"
	"testing"

	"github.com/hanzoai/o11y"
	"github.com/zap-proto/zip"
)

// A DESCRIPTION IS ADDRESSED, and an address nothing declares is not inert.
//
// zip keeps its doc registry as a package-level map keyed by "METHOD path", so a
// Describe outlives the declaration it was generated from. In the community
// binary that only publishes prose for an op that is not there. In a composed
// host it is worse: the host serving that same address gets THIS module's
// description and field names attached to ITS operation, which is a document
// that lies about somebody else's route.
//
// zipdoc regenerates from the registrations, so what it would emit is exactly
// the set below. This is the assertion that the file in the tree still is that.
func TestEveryDescriptionHasAnAddress(t *testing.T) {
	app := zip.New(zip.Config{DisableStartupMessage: true})
	if err := o11y.Use(app); err != nil {
		t.Fatalf("Use: %v", err)
	}
	served := map[string]bool{}
	for _, r := range app.Fiber().GetRoutes(true) {
		if r.Method == "HEAD" || r.Method == "OPTIONS" {
			continue
		}
		served[r.Method+" "+zip.Template(r.Path)] = true
	}

	// zipdoc writes a path parameter as :name; a route table renders it {name}.
	param := regexp.MustCompile(`:([A-Za-z_][A-Za-z0-9_]*)`)

	f, err := parser.ParseFile(token.NewFileSet(), "zipdoc_gen.go", nil, 0)
	if err != nil {
		t.Fatalf("parse zipdoc_gen.go: %v", err)
	}
	described := 0
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) == 0 {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Describe" {
			return true
		}
		lit, ok := call.Args[0].(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		addr, err := strconv.Unquote(lit.Value)
		if err != nil {
			return true
		}
		described++
		if !served[param.ReplaceAllString(addr, "{$1}")] {
			t.Errorf("zipdoc_gen.go describes %q, which this module does not declare", addr)
		}
		return true
	})
	// The other direction. Only a TYPED op carries prose — an escape hatch and a
	// native probe are addresses without a document — so the registry, not the
	// route table, is what a description count has to equal.
	if ops := len(app.Registry()); described != ops {
		t.Errorf("%d descriptions for %d typed ops — a typed op with no prose publishes a bare schema", described, ops)
	}
}
