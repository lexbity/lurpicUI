package platform_test

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPlatformPackages_haveNonEmptyDocGo(t *testing.T) {
	packages := []string{
		".",
		"android",
		"linux",
		filepath.Join("linux", "internal", "display"),
		filepath.Join("linux", "internal", "input"),
		filepath.Join("internal", "common"),
	}

	for _, pkg := range packages {
		path := filepath.Join(pkg, "doc.go")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%s: read doc.go: %v", pkg, err)
		}
		if len(data) == 0 {
			t.Fatalf("%s: doc.go is empty", pkg)
		}
	}
}
