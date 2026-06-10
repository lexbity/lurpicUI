package walk

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/loader"
)

// parseSrc parses a single Go source file and returns the fset and file.
func parseSrc(t *testing.T, src string) (*token.FileSet, *ast.File) {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	return fset, f
}

// firstCompositeLit finds the first composite literal in the AST.
func firstCompositeLit(t *testing.T, f *ast.File) *ast.CompositeLit {
	t.Helper()
	var found *ast.CompositeLit
	ast.Inspect(f, func(n ast.Node) bool {
		if found != nil {
			return false
		}
		if lit, ok := n.(*ast.CompositeLit); ok {
			found = lit
			return false
		}
		return true
	})
	if found == nil {
		t.Fatal("no composite literal found in test source")
	}
	return found
}

// firstFuncLit finds the first function literal in the AST.
func firstFuncLit(t *testing.T, f *ast.File) *ast.FuncLit {
	t.Helper()
	var found *ast.FuncLit
	ast.Inspect(f, func(n ast.Node) bool {
		if found != nil {
			return false
		}
		if fn, ok := n.(*ast.FuncLit); ok {
			found = fn
			return false
		}
		return true
	})
	if found == nil {
		t.Fatal("no function literal found in test source")
	}
	return found
}

func TestSelectorIs(t *testing.T) {
	src := `package p
	var _ = facet.LayoutRole{}
	var _ = gfx.RectFromXYWH(0,0,0,0)
	`
	_, f := parseSrc(t, src)

	calls := FindCallExprs(f, func(call *ast.CallExpr) bool { return true })
	if len(calls) == 0 {
		t.Fatal("expected call expressions")
	}

	// Test SelectorIs on the composite lit type and on call funs.
	ast.Inspect(f, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.CallExpr:
			if SelectorIs(n.Fun, "gfx", "RectFromXYWH") {
				// pass
			}
		}
		return true
	})
}

func TestSelectorIs_True(t *testing.T) {
	src := `package p; var _ = facet.LayoutRole{}`
	_, f := parseSrc(t, src)
	lit := firstCompositeLit(t, f)
	if !SelectorIs(lit.Type, "facet", "LayoutRole") {
		t.Error("SelectorIs(facet.LayoutRole) should be true")
	}
}

func TestSelectorIs_False(t *testing.T) {
	src := `package p; var _ = other.Type{}`
	_, f := parseSrc(t, src)
	lit := firstCompositeLit(t, f)
	if SelectorIs(lit.Type, "facet", "LayoutRole") {
		t.Error("SelectorIs(other.Type) should be false for facet.LayoutRole")
	}
	if SelectorIs(lit.Type, "other", "Other") {
		t.Error("SelectorIs should be false when names mismatch")
	}
}

func TestSelectorIs_WrongPkg(t *testing.T) {
	src := `package p; var _ = gfx.RectFromXYWH(0,0,0,0)`
	_, f := parseSrc(t, src)
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if SelectorIs(call.Fun, "facet", "RectFromXYWH") {
			t.Error("should not match different package")
		}
		if !SelectorIs(call.Fun, "gfx", "RectFromXYWH") {
			t.Error("should match gfx.RectFromXYWH")
		}
		return true
	})
}

func TestCompositeLitIs(t *testing.T) {
	src := `package p; var _ = facet.LayoutRole{OnArrange: nil}`
	_, f := parseSrc(t, src)
	lit := firstCompositeLit(t, f)
	if !CompositeLitIs(lit, "facet", "LayoutRole") {
		t.Error("CompositeLitIs should detect facet.LayoutRole")
	}
}

func TestKeyValue(t *testing.T) {
	src := `package p; var _ = facet.LayoutRole{OnMeasure: nil, OnArrange: nil}`
	_, f := parseSrc(t, src)
	lit := firstCompositeLit(t, f)

	onMeasure := KeyValue(lit, "OnMeasure")
	if onMeasure == nil {
		t.Error("KeyValue(OnMeasure) should not be nil")
	}
	onArrange := KeyValue(lit, "OnArrange")
	if onArrange == nil {
		t.Error("KeyValue(OnArrange) should not be nil")
	}
	missing := KeyValue(lit, "Nonexistent")
	if missing != nil {
		t.Error("KeyValue(Nonexistent) should be nil")
	}
}

func TestFuncLitBody(t *testing.T) {
	src := `package p; var _ = func() {}`
	_, f := parseSrc(t, src)
	fn := firstFuncLit(t, f)
	body := FuncLitBody(fn)
	if body == nil {
		t.Error("FuncLitBody should return non-nil for func literal")
	}
	// Test with non-func expression.
	if FuncLitBody(&ast.Ident{Name: "x"}) != nil {
		t.Error("FuncLitBody should return nil for non-func")
	}
}

func TestEmbedsFacet_True(t *testing.T) {
	src := `package p
	type MyFacet struct {
		facet.Facet
		x int
	}`
	_, f := parseSrc(t, src)
	for _, decl := range f.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range gen.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			imports := loader.BuildImportTable(f.Imports)
			if !EmbedsFacet(ts, imports) {
				t.Error("EmbedsFacet should be true for struct embedding facet.Facet")
			}
		}
	}
}

func TestEmbedsFacet_False(t *testing.T) {
	src := `package p
	type NotAFacet struct {
		x int
	}`
	_, f := parseSrc(t, src)
	for _, decl := range f.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range gen.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			imports := loader.BuildImportTable(f.Imports)
			if EmbedsFacet(ts, imports) {
				t.Error("EmbedsFacet should be false for struct without facet embedding")
			}
		}
	}
}

func TestHasAddRoleCall_True(t *testing.T) {
	src := `package p
	func newFacet() { p.AddRole(&p.layout) }`
	_, f := parseSrc(t, src)
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		if !HasAddRoleCall(fn.Body, "p") {
			t.Error("HasAddRoleCall should find p.AddRole")
		}
	}
}

func TestHasAddRoleCall_False(t *testing.T) {
	src := `package p
	func doSomething() { _ = 1 + 1 }`
	_, f := parseSrc(t, src)
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		if HasAddRoleCall(fn.Body, "p") {
			t.Error("HasAddRoleCall should return false when no AddRole call exists")
		}
	}
}

func TestCountCalls(t *testing.T) {
	src := `package p
	func f() {
		foo.Bar()
		foo.Bar()
		baz.Quux()
	}`
	_, f := parseSrc(t, src)
	var body *ast.BlockStmt
	ast.Inspect(f, func(n ast.Node) bool {
		if fn, ok := n.(*ast.FuncDecl); ok && fn.Name.Name == "f" {
			body = fn.Body
			return false
		}
		return true
	})
	if body == nil {
		t.Fatal("body not found")
	}

	count := CountCalls(body, func(call *ast.CallExpr) bool {
		return SelectorIs(call.Fun, "foo", "Bar")
	})
	if count != 2 {
		t.Errorf("expected 2 foo.Bar() calls, got %d", count)
	}
}

func TestCountRectFromXYWH(t *testing.T) {
	src := `package p
	func f() {
		gfx.RectFromXYWH(0,0,10,10)
		gfx.RectFromXYWH(10,10,20,20)
		_ = 1
	}`
	_, f := parseSrc(t, src)
	var body *ast.BlockStmt
	ast.Inspect(f, func(n ast.Node) bool {
		if fn, ok := n.(*ast.FuncDecl); ok && fn.Name.Name == "f" {
			body = fn.Body
			return false
		}
		return true
	})
	if body == nil {
		t.Fatal("body not found")
	}

	count := CountRectFromXYWH(body, "gfx")
	if count != 2 {
		t.Errorf("expected 2 RectFromXYWH calls, got %d", count)
	}
}

func TestCountArrangedBoundsAssignments(t *testing.T) {
	src := `package p
	func f() {
		p.childA.layout.ArrangedBounds = rect
		p.childB.layout.ArrangedBounds = rect
		_ = 1
	}`
	_, f := parseSrc(t, src)
	var body *ast.BlockStmt
	ast.Inspect(f, func(n ast.Node) bool {
		if fn, ok := n.(*ast.FuncDecl); ok && fn.Name.Name == "f" {
			body = fn.Body
			return false
		}
		return true
	})
	if body == nil {
		t.Fatal("body not found")
	}

	count := CountArrangedBoundsAssignments(body)
	if count != 2 {
		t.Errorf("expected 2 ArrangedBounds assignments, got %d", count)
	}
}

func TestHasRangeOverField_True(t *testing.T) {
	src := `package p
	func f() {
		for _, child := range p.Children {
			_ = child
		}
	}`
	_, f := parseSrc(t, src)
	var body *ast.BlockStmt
	ast.Inspect(f, func(n ast.Node) bool {
		if fn, ok := n.(*ast.FuncDecl); ok && fn.Name.Name == "f" {
			body = fn.Body
			return false
		}
		return true
	})
	if body == nil {
		t.Fatal("body not found")
	}

	if !HasRangeOverField(body, []string{"Children"}) {
		t.Error("HasRangeOverField should detect range over Children")
	}
}

func TestHasRangeOverField_False(t *testing.T) {
	src := `package p
	func f() {
		for i := 0; i < 10; i++ {
			_ = i
		}
	}`
	_, f := parseSrc(t, src)
	var body *ast.BlockStmt
	ast.Inspect(f, func(n ast.Node) bool {
		if fn, ok := n.(*ast.FuncDecl); ok && fn.Name.Name == "f" {
			body = fn.Body
			return false
		}
		return true
	})
	if body == nil {
		t.Fatal("body not found")
	}

	if HasRangeOverField(body, []string{"Children"}) {
		t.Error("HasRangeOverField should return false for regular for loop")
	}
}

func TestCallExprIs(t *testing.T) {
	src := `package p
	var _ = struct{f func()}{f: func() {
		role.Arrange(ctx, bounds)
		role.Measure(ctx, constraints)
		unknown.Call()
	}}`
	_, f := parseSrc(t, src)
	var body *ast.BlockStmt
	ast.Inspect(f, func(n ast.Node) bool {
		if fn, ok := n.(*ast.FuncLit); ok {
			body = fn.Body
			return false
		}
		return true
	})
	if body == nil {
		t.Fatal("func lit body not found")
	}

	// Should detect Arrange and Measure calls.
	arrangeCount := CountCalls(body, func(call *ast.CallExpr) bool {
		return CallExprIs(call, "Arrange", "Measure")
	})
	if arrangeCount != 2 {
		t.Errorf("expected 2 Arrange/Measure calls, got %d", arrangeCount)
	}

	// Should NOT detect unknown.Call().
	unknownCount := CountCalls(body, func(call *ast.CallExpr) bool {
		return CallExprIs(call, "Unknown")
	})
	if unknownCount != 0 {
		t.Errorf("expected 0 Unknown calls, got %d", unknownCount)
	}
}

func TestSelectorChainContains(t *testing.T) {
	src := `package p
	var _ = p.childA.layout.ArrangedBounds
	var _ = p.someOtherField`
	_, f := parseSrc(t, src)
	var foundSel ast.Expr
	ast.Inspect(f, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok && sel.Sel.Name == "ArrangedBounds" {
			foundSel = sel
			return false
		}
		return true
	})
	if foundSel == nil {
		t.Fatal("no ArrangedBounds selector found")
	}

	if !SelectorChainContains(foundSel, "ArrangedBounds") {
		t.Error("SelectorChainContains should match ArrangedBounds at end")
	}
	if SelectorChainContains(foundSel, "Nonexistent") {
		t.Error("SelectorChainContains should not match nonexistent name")
	}
}

func TestPosition(t *testing.T) {
	src := `package p
	var x = 1`
	fset, f := parseSrc(t, src)
	pos := Position(fset, f)
	if pos.Filename != "test.go" {
		t.Errorf("expected filename test.go, got %s", pos.Filename)
	}
}
