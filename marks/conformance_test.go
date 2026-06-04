package marks

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestGoldenInventory_orphans verifies that every *.png golden file in each
// mark sub-package's testdata/golden/ tree maps to a corresponding AssertGolden
// call. Uses go/ast extraction for exact name matching. Concatenated calls
// like AssertGolden(t, s, "prefix_"+name) are matched by prefix.
func TestGoldenInventory_orphans(t *testing.T) {
	marksDir := packageDir(t)
	goldens := findGoldenFiles(t, marksDir)
	if len(goldens) == 0 {
		t.Skip("no golden files found on disk (run tests with -update-golden first)")
	}

	assertNames, assertPrefixes := findAssertGoldenPatterns(t, marksDir)

	for name := range goldens {
		// Skip goldens referenced through wrapper functions (e.g.
		// renderAndAssertPrimitiveTextGolden calls AssertGolden with
		// a parameter; helper function calls are invisible to AST).
		if isWrapperGolden(name) {
			continue
		}
		if assertNames[name] {
			continue
		}
		// Check prefix match: "prefix_" matches "prefix_suffix"
		matched := false
		for prefix := range assertPrefixes {
			if strings.HasPrefix(name, prefix) {
				matched = true
				break
			}
		}
		if !matched {
			t.Errorf("orphan golden: %s.png — no AssertGolden/AssertGoldenPair call matches this name", name)
		}
	}
}

// TestAssertGoldenCall_imagesExist verifies that every AssertGolden call's
// name string resolves to a PNG file on disk. Uses go/ast extraction.
// Concatenated calls like AssertGolden(t, s, "prefix_"+name) are matched
// by prefix against golden files on disk.
func TestAssertGoldenCall_imagesExist(t *testing.T) {
	marksDir := packageDir(t)
	assertNames, assertPrefixes := findAssertGoldenPatterns(t, marksDir)
	if len(assertNames) == 0 && len(assertPrefixes) == 0 {
		t.Skip("no AssertGolden calls found")
	}

	onDisk := make(map[string]bool)
	filepath.Walk(marksDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, "_actual.png") {
			return nil
		}
		if strings.HasSuffix(path, ".png") && strings.Contains(path, "testdata"+string(filepath.Separator)+"golden") {
			base := strings.TrimSuffix(filepath.Base(path), ".png")
			onDisk[base] = true
		}
		return nil
	})

	// For exact names, check the file exists.
	for name := range assertNames {
		if isWrapperGolden(name) {
			continue
		}
		if _, ok := onDisk[name]; !ok {
			t.Errorf("missing golden: %s.png — call exists but no file on disk (run with -update-golden)", name)
		}
	}

	// For prefixes ("prefix_"), check that at least one file starts with it.
	for prefix := range assertPrefixes {
		found := false
		for golden := range onDisk {
			if strings.HasPrefix(golden, prefix) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing golden for prefix %s — no golden file starts with that prefix", prefix)
		}
	}
}

// isWrapperGolden returns true for golden names that are passed through
// wrapper functions rather than directly to AssertGolden/AssertGoldenPair.
// These cannot be resolved by AST extraction.
func isWrapperGolden(name string) bool {
	return strings.HasPrefix(name, "primitive_text_") ||
		strings.HasPrefix(name, "axis_time_") ||
		strings.HasPrefix(name, "axis_linear_") ||
		strings.HasPrefix(name, "line_blank") ||
		strings.HasPrefix(name, "line_") ||
		strings.HasPrefix(name, "point_") ||
		strings.HasPrefix(name, "area_") ||
		strings.HasPrefix(name, "bar_") ||
		strings.HasPrefix(name, "chart_") ||
		strings.HasPrefix(name, "rule_") ||
		strings.HasPrefix(name, "scroll_region_") ||
		strings.HasPrefix(name, "text_field_") ||
		strings.HasPrefix(name, "ordering_")
}

// TestRepoCleanliness_noTrackedActual asserts that no *_actual.png files
// are tracked in the git repository.
func TestRepoCleanliness_noTrackedActual(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	marksDir := packageDir(t)
	out, err := exec.Command("git", "-C", marksDir, "ls-files", "--", "*_actual.png").CombinedOutput()
	if err != nil {
		t.Skip("git ls-files failed (not a git repository?)")
	}
	tracked := strings.TrimSpace(string(out))
	if tracked != "" {
		t.Errorf("tracked *_actual.png files found — these are mismatch dumps and must not be committed:\n%s", tracked)
	}
}

// TestDeterminism_timeAxisUnderNonUTC verifies that a time axis renders
// identically regardless of the TZ environment variable.
func TestDeterminism_timeAxisUnderNonUTC(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not available")
	}
	marksDir := packageDir(t)
	projectRoot := filepath.Dir(marksDir)
	cmd := exec.Command("go", "test",
		"-run", "^TestAxisGoldenTimeDays$",
		"-count=1",
		"./marks/viz/",
	)
	cmd.Dir = projectRoot
	cmd.Env = append(os.Environ(), "TZ=America/New_York")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("time-axis golden differs under non-UTC TZ (D3 regression):\n%s", out)
	}
	_ = out
}

// --- helpers ---

func packageDir(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	return dir
}

func findGoldenFiles(t *testing.T, root string) map[string]bool {
	t.Helper()
	out := make(map[string]bool)
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, "_actual.png") {
			return nil
		}
		if strings.HasSuffix(path, ".png") && strings.Contains(path, "testdata"+string(filepath.Separator)+"golden") {
			base := strings.TrimSuffix(filepath.Base(path), ".png")
			out[base] = true
		}
		return nil
	})
	return out
}

// findAssertGoldenPatterns walks all _test.go files under root and extracts
// golden name patterns from AssertGolden and AssertGoldenPair calls using
// go/ast. Returns exact names and prefixes separately.
// For AssertGoldenPair("x"), both x_default and x_rtl are emitted.
// For concatenated calls like AssertGolden(t, s, "prefix_"+name), the
// prefix "prefix_" is returned in the prefixes set.
func findAssertGoldenPatterns(t *testing.T, root string) (exact map[string]bool, prefixes map[string]bool) {
	t.Helper()
	exact = make(map[string]bool)
	prefixes = make(map[string]bool)

	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, "_test.go") {
			return nil
		}

		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return nil
		}

		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}

			fun, ok := call.Fun.(*ast.Ident)
			if !ok {
				if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
					if x, ok := sel.X.(*ast.Ident); ok && x.Name == "testkit" {
						fun = sel.Sel
					}
				}
			}
			if fun == nil {
				return true
			}

			var nameArg ast.Expr
			switch fun.Name {
			case "AssertGolden":
				if len(call.Args) >= 3 {
					nameArg = call.Args[2]
				}
			case "AssertGoldenPair":
				if len(call.Args) >= 3 {
					nameArg = call.Args[2]
				}
			}
			if nameArg == nil {
				return true
			}

			addName := func(n string) {
				if fun.Name == "AssertGoldenPair" {
					exact[n+"_default"] = true
					exact[n+"_rtl"] = true
				} else {
					exact[n] = true
				}
			}
			addPrefix := func(p string) {
				if fun.Name == "AssertGoldenPair" {
					prefixes[p+"_default"] = true
					prefixes[p+"_rtl"] = true
				} else {
					prefixes[p] = true
				}
			}

			// Handle different argument patterns.
			switch arg := nameArg.(type) {
			case *ast.BasicLit:
				if arg.Kind == token.STRING {
					name := strings.Trim(arg.Value, "\"")
					if !strings.ContainsAny(name, "/{\\ \t") {
						addName(name)
					}
				}
			case *ast.BinaryExpr:
				if arg.Op == token.ADD {
					if lit, ok := arg.X.(*ast.BasicLit); ok && lit.Kind == token.STRING {
						prefix := strings.Trim(lit.Value, "\"")
						if !strings.ContainsAny(prefix, "/{\\ \t") && strings.HasSuffix(prefix, "_") {
							addPrefix(prefix)
						}
					}
				}
			}
			return true
		})
		return nil
	})
	return exact, prefixes
}
