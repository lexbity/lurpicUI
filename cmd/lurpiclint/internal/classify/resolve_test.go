package classify

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"

	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/loader"
)

func parseResolveTestdata(t *testing.T, pkgDir string) *loader.Package {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join("testdata", pkgDir))
	if err != nil {
		t.Fatal(err)
	}
	result, err := loader.Load([]string{abs}, loader.Config{})
	if err != nil {
		t.Fatalf("loading %s: %v", abs, err)
	}
	pkg, ok := result.Packages[abs]
	if !ok {
		t.Fatalf("package %s not found", abs)
	}
	return pkg
}

func parseResolveFile(t *testing.T, pkg, file string) *loader.ParsedFile {
	t.Helper()
	return parseTestdataFile(t, pkg, file)
}

func TestPkgFuncResolver_ResolvesPackageLevelFunc(t *testing.T) {
	pkg := parseResolveTestdata(t, "delegate")
	r := NewPkgFuncResolver(pkg)

	// Find the call to arrangeChildren in the OnArrange lambda.
	pf := pkg.Files[0]
	var callExpr *ast.CallExpr
	ast.Inspect(pf.AST, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		// Look for the arrangeChildren call (ident-based, not selector).
		if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "arrangeChildren" {
			callExpr = call
			return false
		}
		// Also check the method call p.arrangeChildren
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "arrangeChildren" {
			callExpr = call
			return false
		}
		return true
	})
	if callExpr == nil {
		t.Fatal("arrangeChildren call not found in delegate testdata")
	}

	// The call is a method call (p.arrangeChildren(...)), which is resolved by
	// name-only lookup.
	body := r.FuncBody(callExpr)
	if body == nil {
		t.Fatal("FuncBody should resolve method calls by name")
	}
}

func TestPkgFuncResolver_ResolvesDirectCall(t *testing.T) {
	// Test with a synthetic file containing a direct function call.
	src := `package p
		func childArranger(a, b int) {}
		func caller() { childArranger(1, 2) }`

	fset := token.NewFileSet()
	astFile, err := parser.ParseFile(fset, "", src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	pkg := &loader.Package{
		Name: "p",
		Path: "/test/p",
		Files: []*loader.ParsedFile{
			{
				Fset: fset,
				AST:  astFile,
				Path: "/test/p/pkg.go",
				Pkg:  "p",
			},
		},
	}
	r := NewPkgFuncResolver(pkg)

	var callExpr *ast.CallExpr
	ast.Inspect(astFile, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "childArranger" {
			callExpr = call
			return false
		}
		return true
	})
	if callExpr == nil {
		t.Fatal("childArranger call not found")
	}

	body := r.FuncBody(callExpr)
	if body == nil {
		t.Fatal("FuncBody returned nil for direct call to childArranger")
	}
}

func TestPkgFuncResolver_UnknownFuncReturnsNil(t *testing.T) {
	pkg := parseResolveTestdata(t, "delegate")
	r := NewPkgFuncResolver(pkg)

	// A call to a non-existent function should return nil.
	callExpr := &ast.CallExpr{
		Fun: &ast.Ident{Name: "nonexistent"},
	}
	if body := r.FuncBody(callExpr); body != nil {
		t.Error("expected nil for unknown function")
	}
}

func TestPkgFuncResolver_NonIdentCallReturnsNil(t *testing.T) {
	pkg := parseResolveTestdata(t, "delegate")
	r := NewPkgFuncResolver(pkg)

	// A call via selector (x.Method()) should return nil.
	callExpr := &ast.CallExpr{
		Fun: &ast.SelectorExpr{
			X:   &ast.Ident{Name: "x"},
			Sel: &ast.Ident{Name: "Method"},
		},
	}
	if body := r.FuncBody(callExpr); body != nil {
		t.Error("expected nil for selector-based call")
	}
}
