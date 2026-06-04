package marks

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestGoldenInventory_orphans verifies that every *.png golden file in each
// mark sub-package's testdata/golden/ tree maps to a corresponding AssertGolden
// call. Uses a glob-based convention: the golden base name must contain a
// substring that matches an entry in our known test name patterns.
func TestGoldenInventory_orphans(t *testing.T) {
	marksDir := packageDir(t)
	goldens := findGoldenFiles(t, marksDir)
	if len(goldens) == 0 {
		t.Skip("no golden files found on disk (run tests with -update-golden first)")
	}

	assertCalls := findAssertGoldenCalls(t, marksDir)

	// For each golden, check that its base name appears as a substring
	// in at least one AssertGolden call's full golden name argument.
	// This is a fuzzy match: we extract the golden name from calls
	// and check if the golden name matches any known form.
	for name := range goldens {
		found := false
		for call := range assertCalls {
			if strings.Contains(call, name) || strings.Contains(name, call) {
				found = true
				break
			}
		}
		if !found {
			// Some goldens use a prefix pattern: mark_prefix_variant.
			// Try matching against known mark prefixes.
			parts := strings.SplitN(name, "_", 2)
			if len(parts) == 2 {
				for call := range assertCalls {
					if strings.HasPrefix(call, parts[0]) {
						found = true
						break
					}
				}
			}
		}
		if !found {
			t.Errorf("orphan golden: %s.png — no AssertGolden call matches this name", name)
		}
	}
}

// TestAssertGoldenCall_imagesExist verifies that every AssertGolden call's
// name string resolves to a PNG file on disk. Skips calls that use string
// concatenation or variables (we can't statically resolve those).
func TestAssertGoldenCall_imagesExist(t *testing.T) {
	marksDir := packageDir(t)
	assertCalls := findAssertGoldenCalls(t, marksDir)
	if len(assertCalls) == 0 {
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

	for name := range assertCalls {
		// Skip names that are clearly concatenation prefixes or parsing artifacts.
		if strings.HasSuffix(name, "_") ||
			strings.Contains(name, "+") ||
			strings.Contains(name, "fmt.") ||
			strings.Contains(name, "%s") ||
			strings.Contains(name, " ") ||
			strings.HasPrefix(name, "testkit.") {
			continue
		}
		if _, ok := onDisk[name]; !ok {
			t.Errorf("missing golden: %s.png — call exists but no file on disk (run with -update-golden)", name)
		}
	}
}

// TestRepoCleanliness_noTrackedActual asserts that no *_actual.png files
// are tracked in the git repository. Such files indicate that a golden test
// ran with mismatched output and the mismatch dump was accidentally committed.
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

func findAssertGoldenCalls(t *testing.T, root string) map[string]bool {
	t.Helper()
	out := make(map[string]bool)
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		content := string(data)
		for _, prefix := range []string{"AssertGolden(t,", "testkit.AssertGolden(t,"} {
			idx := 0
			for {
				pos := strings.Index(content[idx:], prefix)
				if pos < 0 {
					break
				}
				start := idx + pos + len(prefix)
				commaCount := 0
				for i := start; i < len(content); i++ {
					if content[i] == '"' && commaCount >= 1 {
						end := strings.IndexByte(content[i+1:], '"')
						if end >= 0 {
							name := content[i+1 : i+1+end]
							if !strings.Contains(name, "/") && !strings.Contains(name, "{") && !strings.Contains(name, "\\") {
								out[name] = true
							}
						}
						break
					}
					if content[i] == ',' {
						commaCount++
					}
				}
				idx = idx + pos + 1
			}
		}
		return nil
	})
	return out
}
