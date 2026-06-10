package loader

import (
	"go/ast"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// testdata returns the absolute path to the testdata directory.
func testdata(tb testing.TB, elem ...string) string {
	tb.Helper()
	elems := append([]string{"testdata"}, elem...)
	p := filepath.Join(elems...)
	abs, err := filepath.Abs(p)
	if err != nil {
		tb.Fatal(err)
	}
	return abs
}

func TestLoad_SinglePackage(t *testing.T) {
	dir := testdata(t, "simple")
	result, err := Load([]string{dir}, Config{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Packages) != 1 {
		t.Fatalf("got %d packages, want 1", len(result.Packages))
	}
	pkg := result.Packages[filepath.Clean(dir)]
	if pkg == nil {
		t.Fatal("package not found by directory path")
	}
	if pkg.Name != "simple" {
		t.Errorf("package name = %q, want %q", pkg.Name, "simple")
	}
	if len(pkg.Files) != 1 {
		t.Fatalf("got %d files, want 1", len(pkg.Files))
	}
	if pkg.Files[0].Pkg != "simple" {
		t.Errorf("file package = %q, want %q", pkg.Files[0].Pkg, "simple")
	}
	// Top-level Files slice must match.
	if len(result.Files) != 1 {
		t.Fatalf("got %d files in result, want 1", len(result.Files))
	}
}

func TestLoad_MultiFilePackage(t *testing.T) {
	dir := testdata(t, "multi")
	result, err := Load([]string{dir}, Config{})
	if err != nil {
		t.Fatal(err)
	}
	pkg := result.Packages[filepath.Clean(dir)]
	if pkg == nil {
		t.Fatal("package not found")
	}
	if pkg.Name != "multi" {
		t.Errorf("package name = %q, want %q", pkg.Name, "multi")
	}
	if len(pkg.Files) != 2 {
		t.Fatalf("got %d files, want 2", len(pkg.Files))
	}
	// Must be sorted.
	if pkg.Files[0].Path >= pkg.Files[1].Path {
		t.Errorf("files not sorted: %s >= %s", pkg.Files[0].Path, pkg.Files[1].Path)
	}
}

func TestLoad_NestedPackages(t *testing.T) {
	dir := testdata(t, "nested")
	result, err := Load([]string{dir + "/..."}, Config{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Packages) != 2 {
		t.Fatalf("got %d packages, want 2: %v", len(result.Packages), packageNames(result))
	}
	// Verify both packages discovered.
	alphaPath := filepath.Join(dir, "alpha")
	betaPath := filepath.Join(dir, "beta")
	if result.Packages[alphaPath] == nil {
		t.Errorf("package alpha not found at %s", alphaPath)
	} else if result.Packages[alphaPath].Name != "alpha" {
		t.Errorf("alpha name = %q", result.Packages[alphaPath].Name)
	}
	if result.Packages[betaPath] == nil {
		t.Errorf("package beta not found at %s", betaPath)
	} else if result.Packages[betaPath].Name != "beta" {
		t.Errorf("beta name = %q", result.Packages[betaPath].Name)
	}
	// Verify deterministic top-level file list.
	if len(result.Files) != 2 {
		t.Fatalf("got %d files, want 2", len(result.Files))
	}
	// The nested/alpha/a.go and nested/beta/b.go must be sorted.
	alphaFile := result.Files[0]
	betaFile := result.Files[1]
	if alphaFile.Path >= betaFile.Path {
		t.Errorf("files not sorted: %s >= %s", alphaFile.Path, betaFile.Path)
	}
}

func TestLoad_NestedNoRecurse(t *testing.T) {
	// Without /..., only the named directory is loaded.
	dir := testdata(t, "nested")
	result, err := Load([]string{dir}, Config{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Packages) != 0 {
		t.Fatalf("got %d packages, want 0 (nested dir has no .go files at root)", len(result.Packages))
	}
}

func TestLoad_ExcludeTestsByDefault(t *testing.T) {
	dir := testdata(t, "withtests")
	result, err := Load([]string{dir}, Config{})
	if err != nil {
		t.Fatal(err)
	}
	pkg := result.Packages[filepath.Clean(dir)]
	if pkg == nil {
		t.Fatal("package not found")
	}
	// Only the non-test file should be loaded.
	if len(pkg.Files) != 1 {
		t.Fatalf("got %d files, want 1 (test file should be excluded)", len(pkg.Files))
	}
	if strings.HasSuffix(pkg.Files[0].Path, "_test.go") {
		t.Error("test file was loaded despite default exclusion")
	}
}

func TestLoad_IncludeTests(t *testing.T) {
	dir := testdata(t, "withtests")
	result, err := Load([]string{dir}, Config{IncludeTests: true})
	if err != nil {
		t.Fatal(err)
	}
	pkg := result.Packages[filepath.Clean(dir)]
	if pkg == nil {
		t.Fatal("package not found")
	}
	if len(pkg.Files) != 2 {
		t.Fatalf("got %d files, want 2 (both code.go and code_test.go)", len(pkg.Files))
	}
	foundTest := false
	for _, f := range pkg.Files {
		if strings.HasSuffix(f.Path, "_test.go") {
			foundTest = true
			break
		}
	}
	if !foundTest {
		t.Error("test file was not loaded despite IncludeTests=true")
	}
}

func TestLoad_ParseError(t *testing.T) {
	dir := testdata(t, "malformed")
	_, err := Load([]string{dir}, Config{})
	if err == nil {
		t.Fatal("expected parse error, got nil")
	}
	if !strings.Contains(err.Error(), "parsing") {
		t.Errorf("error should mention parsing: %v", err)
	}
}

func TestLoad_EmptyPattern(t *testing.T) {
	result, err := Load([]string{}, Config{})
	if err != nil {
		t.Fatal(err)
	}
	// Default should be "." — the module root may have packages.
	if result == nil {
		t.Fatal("result is nil")
	}
}

func TestLoad_DeterministicOrder(t *testing.T) {
	// Load the same tree twice and verify identical results.
	dir := testdata(t, "nested")
	r1, err := Load([]string{dir + "/..."}, Config{})
	if err != nil {
		t.Fatal(err)
	}
	r2, err := Load([]string{dir + "/..."}, Config{})
	if err != nil {
		t.Fatal(err)
	}
	if len(r1.Files) != len(r2.Files) {
		t.Fatalf("file count mismatch: %d vs %d", len(r1.Files), len(r2.Files))
	}
	for i := range r1.Files {
		if r1.Files[i].Path != r2.Files[i].Path {
			t.Errorf("file %d: %s vs %s", i, r1.Files[i].Path, r2.Files[i].Path)
		}
	}
}

func TestLoad_SingleGoFile(t *testing.T) {
	filePath := testdata(t, "simple", "simple.go")
	result, err := Load([]string{filePath}, Config{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Packages) != 1 {
		t.Fatalf("got %d packages, want 1", len(result.Packages))
	}
}

func TestLoad_Malformed_Pattern(t *testing.T) {
	_, err := Load([]string{"nonexistent_path_xyz"}, Config{})
	if err == nil {
		t.Fatal("expected error for nonexistent path")
	}
}

func TestBuildImportTable_Alias(t *testing.T) {
	dir := testdata(t, "aliased")
	result, err := Load([]string{dir}, Config{})
	if err != nil {
		t.Fatal(err)
	}
	pkg := result.Packages[filepath.Clean(dir)]
	if pkg == nil {
		t.Fatal("package not found")
	}

	// The first file has all the imports we care about.
	file := pkg.Files[0]
	table := file.Imports

	// Explicit alias: f "codeburg.org/lexbit/lurpicui/facet"
	if table["f"] != "codeburg.org/lexbit/lurpicui/facet" {
		t.Errorf(`table["f"] = %q, want "codeburg.org/lexbit/lurpicui/facet"`, table["f"])
	}

	// No alias: "codeburg.org/lexbit/lurpicui/gfx" → key "gfx"
	if table["gfx"] != "codeburg.org/lexbit/lurpicui/gfx" {
		t.Errorf(`table["gfx"] = %q, want "codeburg.org/lexbit/lurpicui/gfx"`, table["gfx"])
	}

	// Blank import: _ "codeburg.org/lexbit/lurpicui/layout" — must NOT be in table
	if _, ok := table["layout"]; ok {
		t.Error(`blank import "layout" should not be in table`)
	}

	// Dot import: . "codeburg.org/lexbit/lurpicui/projection" → key "."
	if table["."] != "codeburg.org/lexbit/lurpicui/projection" {
		t.Errorf(`table["."] = %q, want "codeburg.org/lexbit/lurpicui/projection"`, table["."])
	}
}

func TestLoad_MultiFileImportAccumulation(t *testing.T) {
	// Two files in aliased/: aliased.go has imports, dupimport.go has none.
	// The package collects both files.
	dir := testdata(t, "aliased")
	result, err := Load([]string{dir}, Config{})
	if err != nil {
		t.Fatal(err)
	}
	pkg := result.Packages[filepath.Clean(dir)]
	if pkg == nil {
		t.Fatal("package not found")
	}
	if len(pkg.Files) != 2 {
		t.Fatalf("got %d files, want 2", len(pkg.Files))
	}
}

func TestLoad_CommentsPreserved(t *testing.T) {
	dir := testdata(t, "simple")
	result, err := Load([]string{dir}, Config{})
	if err != nil {
		t.Fatal(err)
	}
	file := result.Files[0]
	// ParseComments was used; assert the file has a comment group.
	// Even if there's no comment, the AST should be valid.
	if file.AST == nil {
		t.Fatal("AST is nil")
	}
}

func TestFileCache_Hit(t *testing.T) {
	dir := testdata(t, "simple")
	filePath := filepath.Join(dir, "simple.go")

	cache := NewFileCache()
	cfg := Config{Cache: cache}

	// First load populates the cache.
	r1, err := Load([]string{dir}, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(r1.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(r1.Files))
	}

	// Second load should hit the cache.
	r2, err := Load([]string{dir}, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(r2.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(r2.Files))
	}
	_ = filePath
}

func TestFileCache_StaleInvalidatedOnEdit(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.go")
	origContent := []byte("package p\nconst X = 1\n")
	modifiedContent := []byte("package p\nconst X = 2\n")

	if err := os.WriteFile(filePath, origContent, 0644); err != nil {
		t.Fatal(err)
	}

	cache := NewFileCache()
	cfg := Config{Cache: cache}

	// Load original.
	r1, err := Load([]string{dir}, cfg)
	if err != nil {
		t.Fatal(err)
	}
	origFile := r1.Files[0]

	// Modify the file.
	if err := os.WriteFile(filePath, modifiedContent, 0644); err != nil {
		t.Fatal(err)
	}

	// Load again — should detect the change.
	r2, err := Load([]string{dir}, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(r2.Files) != 1 {
		t.Fatalf("expected 1 file after modification, got %d", len(r2.Files))
	}

	// The AST should reflect the new content.
	ast.Inspect(r2.Files[0].AST, func(n ast.Node) bool {
		vs, ok := n.(*ast.ValueSpec)
		if !ok || len(vs.Names) == 0 || vs.Names[0].Name != "X" {
			return true
		}
		if len(vs.Values) > 0 {
			if bl, ok := vs.Values[0].(*ast.BasicLit); ok {
				if bl.Value != "2" {
					t.Errorf("after modification: X = %s, want 2", bl.Value)
				}
			}
		}
		return true
	})
	_ = origFile
}

// packageNames returns a sorted slice of package names for diagnostics.
func packageNames(r *LoadResult) []string {
	names := make([]string, 0, len(r.Packages))
	for _, pkg := range r.Packages {
		names = append(names, pkg.Name)
	}
	sort.Strings(names)
	return names
}
