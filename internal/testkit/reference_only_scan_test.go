package testkit

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRepository_has_no_reference_only_dependencies(t *testing.T) {
	root := repoRoot(t)
	forbidden := []string{"REFERENCE_ONLY", "reference-only", "reference only"}
	scanDir(t, root, func(path string, data []byte) {
		rel, err := filepath.Rel(root, path)
		if err != nil {
			t.Fatalf("rel path: %v", err)
		}
		if rel == filepath.Join("internal", "testkit", "reference_only_scan_test.go") {
			return
		}
		if strings.HasPrefix(rel, "devdocs"+string(filepath.Separator)) {
			return
		}
		for _, term := range forbidden {
			if bytesContainsFold(data, term) {
				t.Fatalf("reference-only dependency found in %s", rel)
			}
		}
	})
}

func repoRoot(t testing.TB) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func scanDir(t testing.TB, root string, fn func(path string, data []byte)) {
	t.Helper()
	allowed := map[string]struct{}{
		".go":    {},
		".md":    {},
		".txt":   {},
		".json":  {},
		".toml":  {},
		".cmake": {},
		".in":    {},
		".mod":   {},
		".sum":   {},
	}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			base := filepath.Base(path)
			if base == ".git" || base == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if _, ok := allowed[filepath.Ext(path)]; !ok && filepath.Base(path) != "README" && filepath.Base(path) != "README.md" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		fn(path, data)
		return nil
	})
	if err != nil {
		t.Fatalf("walk repo: %v", err)
	}
}

func bytesContainsFold(data []byte, term string) bool {
	if len(data) == 0 || term == "" {
		return false
	}
	haystack := strings.ToLower(string(data))
	needle := strings.ToLower(term)
	return strings.Contains(haystack, needle)
}
