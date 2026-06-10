package classify

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"

	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/loader"
	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/walk"
)

// parseTestdataFile parses a single Go source file from testdata and returns
// a ParsedFile suitable for classifier tests.
func parseTestdataFile(t *testing.T, pkg, filename string) *loader.ParsedFile {
	t.Helper()
	path := filepath.Join("testdata", pkg, filename)
	fset := token.NewFileSet()
	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	astFile, err := parser.ParseFile(fset, abs, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}
	return &loader.ParsedFile{
		Fset:    fset,
		AST:     astFile,
		Path:    abs,
		Pkg:     astFile.Name.Name,
		Imports: loader.BuildImportTable(astFile.Imports),
	}
}

// findLayoutRoles returns all facet.LayoutRole composite literals in the file.
func findLayoutRoles(t *testing.T, pf *loader.ParsedFile) []*ast.CompositeLit {
	t.Helper()
	var lits []*ast.CompositeLit
	fid := facetIdent(pf.Imports)
	ast.Inspect(pf.AST, func(n ast.Node) bool {
		lit, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}
		if walk.CompositeLitIs(lit, fid, "LayoutRole") {
			lits = append(lits, lit)
		}
		return true
	})
	return lits
}

func TestIsChildArranging_Leaf(t *testing.T) {
	pf := parseTestdataFile(t, "leaf", "leaf.go")
	for _, lit := range findLayoutRoles(t, pf) {
		if IsChildArranging(lit, pf.Fset, pf.Imports) {
			t.Error("leaf LayoutRole should NOT be child-arranging")
		}
	}
}

func TestIsChildArranging_Root(t *testing.T) {
	pf := parseTestdataFile(t, "root", "root.go")
	for _, lit := range findLayoutRoles(t, pf) {
		if !IsChildArranging(lit, pf.Fset, pf.Imports) {
			t.Error("root LayoutRole SHOULD be child-arranging")
		}
	}
}

func TestIsChildArranging_Alias(t *testing.T) {
	pf := parseTestdataFile(t, "alias", "alias.go")
	for _, lit := range findLayoutRoles(t, pf) {
		if !IsChildArranging(lit, pf.Fset, pf.Imports) {
			t.Error("alias LayoutRole (with aliased imports) SHOULD be child-arranging")
		}
	}
}

func TestIsChildArranging_Delegate(t *testing.T) {
	pf := parseTestdataFile(t, "delegate", "delegate.go")
	for _, lit := range findLayoutRoles(t, pf) {
		// The delegate fixture's OnArrange lambda only calls a helper
		// method.  Phase 4 does not trace into helper functions.
		if IsChildArranging(lit, pf.Fset, pf.Imports) {
			t.Error("delegate LayoutRole should NOT be child-arranging (helper delegation not traced)")
		}
	}
}

func TestIsChildArranging_RangeChildren(t *testing.T) {
	pf := parseTestdataFile(t, "rangechildren", "range.go")
	for _, lit := range findLayoutRoles(t, pf) {
		if !IsChildArranging(lit, pf.Fset, pf.Imports) {
			t.Error("rangechildren LayoutRole SHOULD be child-arranging (range over Children)")
		}
	}
}

func TestIsChildArranging_MultiRect(t *testing.T) {
	pf := parseTestdataFile(t, "multi_rect", "rect.go")
	for _, lit := range findLayoutRoles(t, pf) {
		if !IsChildArranging(lit, pf.Fset, pf.Imports) {
			t.Error("multi_rect LayoutRole SHOULD be child-arranging (2+ RectFromXYWH)")
		}
	}
}

func TestIsChildArranging_OneArrangeOneRect(t *testing.T) {
	pf := parseTestdataFile(t, "one_arrange_one_rect", "one.go")
	for _, lit := range findLayoutRoles(t, pf) {
		if !IsChildArranging(lit, pf.Fset, pf.Imports) {
			t.Error("one_arrange_one_rect LayoutRole SHOULD be child-arranging (1 Arrange + 1 RectFromXYWH)")
		}
	}
}

func TestIsChildArranging_MultiArrangedBounds(t *testing.T) {
	pf := parseTestdataFile(t, "multi_arrangedbounds", "bounds.go")
	for _, lit := range findLayoutRoles(t, pf) {
		if !IsChildArranging(lit, pf.Fset, pf.Imports) {
			t.Error("multi_arrangedbounds LayoutRole SHOULD be child-arranging (2+ ArrangedBounds)")
		}
	}
}

func TestIsAuthoring_Leaf(t *testing.T) {
	pf := parseTestdataFile(t, "leaf", "leaf.go")
	for _, decl := range pf.AST.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.TYPE {
			continue
		}
		for _, spec := range gen.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			if !walk.EmbedsFacet(ts, pf.Imports) {
				t.Errorf("LeafPane should embed facet.Facet")
			}
		}
	}
}

func TestIsAuthoring_Root(t *testing.T) {
	pf := parseTestdataFile(t, "root", "root.go")
	found := false
	for _, decl := range pf.AST.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.TYPE {
			continue
		}
		for _, spec := range gen.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			if walk.EmbedsFacet(ts, pf.Imports) {
				found = true
			}
		}
	}
	if !found {
		t.Error("RootFacet should embed facet.Facet")
	}
}

func TestClassifyDecl_Leaf(t *testing.T) {
	pf := parseTestdataFile(t, "leaf", "leaf.go")
	for _, decl := range pf.AST.Decls {
		act := ClassifyDecl(decl, pf.Fset, pf.Imports)
		if act == ActivityAuthoring {
			return // found at least one authoring decl
		}
	}
	t.Error("expected at least one authoring declaration in leaf testdata")
}

func TestActivity_String(t *testing.T) {
	tests := []struct {
		a    Activity
		want string
	}{
		{ActivityUnknown, "unknown"},
		{ActivityAuthoring, "authoring"},
		{ActivityComposition, "composition"},
		{Activity(99), "activity(?)"},
	}
	for _, tt := range tests {
		if tt.a.String() != tt.want {
			t.Errorf("Activity(%d).String() = %q, want %q", tt.a, tt.a.String(), tt.want)
		}
	}
}

func TestClassifyDecl_ReturnsUnknownForNonTypeDecl(t *testing.T) {
	src := `package p
	var x = 1`
	fset := token.NewFileSet()
	astFile, err := parser.ParseFile(fset, "", src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	imports := loader.BuildImportTable(astFile.Imports)
	for _, decl := range astFile.Decls {
		act := ClassifyDecl(decl, fset, imports)
		if act != ActivityUnknown {
			t.Errorf("expected ActivityUnknown for var decl, got %s", act)
		}
	}
}

func TestClassifyDecl_ReturnsUnknownForNonEmbedding(t *testing.T) {
	src := `package p
	type NotFacet struct { x int }`
	fset := token.NewFileSet()
	astFile, err := parser.ParseFile(fset, "", src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	imports := loader.BuildImportTable(astFile.Imports)
	for _, decl := range astFile.Decls {
		act := ClassifyDecl(decl, fset, imports)
		if act != ActivityUnknown {
			t.Errorf("expected ActivityUnknown for non-embedding type, got %s", act)
		}
	}
}

func TestLocalIdent_ResolvesAlias(t *testing.T) {
	imports := loader.ImportTable{
		"f":  "codeburg.org/lexbit/lurpicui/facet",
		"gg": "codeburg.org/lexbit/lurpicui/gfx",
	}
	if got := localIdent(imports, "facet", "facet"); got != "f" {
		t.Errorf("localIdent(facet) = %q, want %q", got, "f")
	}
	if got := localIdent(imports, "gfx", "gfx"); got != "gg" {
		t.Errorf("localIdent(gfx) = %q, want %q", got, "gg")
	}
}

func TestLocalIdent_Fallback(t *testing.T) {
	imports := loader.ImportTable{}
	if got := localIdent(imports, "gfx", "gfx"); got != "gfx" {
		t.Errorf("localIdent with empty imports = %q, want %q", got, "gfx")
	}
}
