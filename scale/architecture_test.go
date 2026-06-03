package scale

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"
)

// forbiddenImports is the set of packages that the core scale package must
// never import. This mechanically enforces Design Principle P1 (pure core).
var forbiddenImports = map[string]string{
	"codeburg.org/lexbit/lurpicui/gfx":     "P1 violated: scale must not import gfx (use float64 internally; narrow at the adapter boundary)",
	"codeburg.org/lexbit/lurpicui/facet":   "P1 violated: scale must not import facet",
	"codeburg.org/lexbit/lurpicui/layout":  "P1 violated: scale must not import layout",
	"codeburg.org/lexbit/lurpicui/runtime": "P1 violated: scale must not import runtime",
	"codeburg.org/lexbit/lurpicui/store":   "P1 violated: scale must not import store (the reactive layer is in scale/reactive)",
}

func TestNoForbiddenImports(t *testing.T) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", nil, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}

	for _, pkg := range pkgs {
		for filename, f := range pkg.Files {
			for _, imp := range f.Imports {
				path := imp.Path.Value
				path = path[1 : len(path)-1] // strip quotes
				if msg, ok := forbiddenImports[path]; ok {
					t.Errorf("%s (imported in %s at %s)",
						msg, filepath.Base(filename), fset.Position(imp.Pos()))
				}
			}
		}
	}
}
