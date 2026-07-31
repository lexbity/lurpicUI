package contracttest

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"testing"
)

// helperExists checks whether a function with the given name is declared
// in the contracttest package source (excluding _test.go files).
func helperExists(name string) bool {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi os.FileInfo) bool {
		return !isTestFile(fi.Name())
	}, parser.AllErrors)
	if err != nil {
		return false
	}
	for _, pkg := range pkgs {
		for _, f := range pkg.Files {
			for _, decl := range f.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok {
					continue
				}
				if fn.Name.Name == name {
					return true
				}
			}
		}
	}
	return false
}

func isTestFile(name string) bool {
	return len(name) >= 8 && name[len(name)-8:] == "_test.go"
}

func TestEveryCapabilityHasContractHelper(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "../capabilities.go", nil, parser.AllErrors)
	if err != nil {
		t.Fatalf("failed to parse marks/capabilities.go: %v", err)
	}

	allowlist := map[string]string{
		"Customizable":    "reserved for future use per capabilities.go:21",
		"Encoding":        "viz encoding channel interface, out of catalog (read-mostly) scope",
		"AnchorExporting": "type alias for layout.AnchorExporter; tested via AssertAnchorExport",
	}

	for _, decl := range f.Decls {
		gs, ok := decl.(*ast.GenDecl)
		if !ok || gs.Tok != token.TYPE {
			continue
		}
		for _, spec := range gs.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			name := ts.Name.Name
			if name[0] < 'A' || name[0] > 'Z' {
				continue
			}
			if _, ok := ts.Type.(*ast.InterfaceType); !ok {
				continue
			}
			if _, allowed := allowlist[name]; allowed {
				continue
			}
			helperName := "Assert" + name
			if !helperExists(helperName) {
				t.Errorf("capability %q has no contracttest.%s — add one or extend the allowlist", name, helperName)
			}
		}
	}
}
