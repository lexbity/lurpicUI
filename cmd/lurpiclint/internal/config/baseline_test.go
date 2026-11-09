package config

import (
	"encoding/json"
	"go/token"
	"os"
	"path/filepath"
	"testing"

	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/diag"
)

// symlinkResolved returns dir with symlinks resolved, matching how
// ModuleRoot and RelPath canonicalize paths.
func symlinkResolved(t *testing.T, dir string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatalf("EvalSymlinks(%s): %v", dir, err)
	}
	return resolved
}

func TestModuleRoot(t *testing.T) {
	tmp := t.TempDir()
	root := symlinkResolved(t, tmp)
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/x\n"), 0600); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(root, "sub", "pkg")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatal(err)
	}

	if got := ModuleRoot(nested); got != root {
		t.Errorf("ModuleRoot(%s) = %q, want %q", nested, got, root)
	}
	if got := ModuleRoot(root); got != root {
		t.Errorf("ModuleRoot(%s) = %q, want %q", root, got, root)
	}

	// A directory tree with no go.mod anywhere above it yields "".
	// t.TempDir() lives under the system temp dir, which has no go.mod.
	orphan := t.TempDir()
	if got := ModuleRoot(orphan); got != "" {
		t.Errorf("ModuleRoot(%s) = %q, want \"\" (no go.mod above)", orphan, got)
	}
}

func TestRelPath(t *testing.T) {
	root := symlinkResolved(t, t.TempDir())
	sep := string(os.PathSeparator)

	tests := []struct {
		name string
		root string
		path string
		want string
	}{
		{"abs under root", root, root + sep + "marks" + sep + "data" + sep + "x.go", "marks/data/x.go"},
		{"relative unchanged", root, "marks/data/x.go", "marks/data/x.go"},
		{"foreign abs unchanged", root, "/foreign/machine/marks/x.go", "/foreign/machine/marks/x.go"},
		{"empty root", "", "/any/abs/x.go", "/any/abs/x.go"},
		{"empty path", root, "", ""},
		{"root itself", root, root, "."},
		{"root prefix is not a partial match", root, root + "-suffix/x.go", root + "-suffix/x.go"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RelPath(tt.root, tt.path); got != tt.want {
				t.Errorf("RelPath(%q, %q) = %q, want %q", tt.root, tt.path, got, tt.want)
			}
		})
	}
}

func TestLoadBaseline_RelativizesAbsoluteEntries(t *testing.T) {
	root := symlinkResolved(t, t.TempDir())
	sep := string(os.PathSeparator)
	path := filepath.Join(t.TempDir(), "baseline.json")

	raw := Baseline{Entries: []BaselineEntry{
		{RuleID: "LL012", File: root + sep + "marks" + sep + "data" + sep + "join_test.go", Line: 16, Message: "m1"},
		{RuleID: "LL001", File: "/foreign/machine/a.go", Line: 3, Message: "m2"},
		{RuleID: "LL003", File: "relative/b.go", Line: 7, Message: "m3"},
	}}
	data, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}

	b, err := LoadBaseline(path, root)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"marks/data/join_test.go", "/foreign/machine/a.go", "relative/b.go"}
	if len(b.Entries) != len(want) {
		t.Fatalf("got %d entries, want %d", len(b.Entries), len(want))
	}
	for i, w := range want {
		if b.Entries[i].File != w {
			t.Errorf("entry %d File = %q, want %q", i, b.Entries[i].File, w)
		}
	}
}

// TestSuppressByBaseline_PathIndependent is the FR-10 teeth test: suppression
// must depend only on the repo-relative path, never on where the module is
// checked out.
func TestSuppressByBaseline_PathIndependent(t *testing.T) {
	root := symlinkResolved(t, t.TempDir())
	sep := string(os.PathSeparator)
	absFile := root + sep + "marks" + sep + "x.go"

	mkDiag := func(line int, msg string) *diag.Diagnostic {
		return &diag.Diagnostic{RuleID: "LL012", Pos: token.Position{Filename: absFile, Line: line}, Message: msg}
	}

	t.Run("relative entry suppresses absolute diagnostic under same root", func(t *testing.T) {
		baseline := &Baseline{Entries: []BaselineEntry{
			{RuleID: "LL012", File: "marks/x.go", Line: 5, Message: "m"},
		}}
		filtered, stale := SuppressByBaseline(root, []*diag.Diagnostic{mkDiag(5, "m")}, baseline)
		if len(filtered) != 0 {
			t.Errorf("filtered = %d, want 0 (suppressed)", len(filtered))
		}
		if len(stale) != 0 {
			t.Errorf("stale = %d, want 0", len(stale))
		}
	})

	t.Run("absolute entry with matching root prefix suppresses (load-time relativization)", func(t *testing.T) {
		baseline := &Baseline{Entries: []BaselineEntry{
			{RuleID: "LL012", File: absFile, Line: 5, Message: "m"},
		}}
		// Relativize the entry the way LoadBaseline does.
		for i := range baseline.Entries {
			baseline.Entries[i].File = RelPath(root, baseline.Entries[i].File)
		}
		filtered, stale := SuppressByBaseline(root, []*diag.Diagnostic{mkDiag(5, "m")}, baseline)
		if len(filtered) != 0 || len(stale) != 0 {
			t.Errorf("filtered=%d stale=%d, want 0/0", len(filtered), len(stale))
		}
	})

	t.Run("entry under a foreign root is stale, not a match", func(t *testing.T) {
		foreign := symlinkResolved(t, t.TempDir())
		baseline := &Baseline{Entries: []BaselineEntry{
			{RuleID: "LL012", File: foreign + sep + "marks" + sep + "x.go", Line: 5, Message: "m"},
		}}
		filtered, stale := SuppressByBaseline(root, []*diag.Diagnostic{mkDiag(5, "m")}, baseline)
		if len(filtered) != 1 {
			t.Errorf("filtered = %d, want 1 (must NOT suppress across roots)", len(filtered))
		}
		if len(stale) != 1 {
			t.Errorf("stale = %d, want 1", len(stale))
		}
	})

	t.Run("line mismatch does not suppress", func(t *testing.T) {
		baseline := &Baseline{Entries: []BaselineEntry{
			{RuleID: "LL012", File: "marks/x.go", Line: 99, Message: "m"},
		}}
		filtered, stale := SuppressByBaseline(root, []*diag.Diagnostic{mkDiag(5, "m")}, baseline)
		if len(filtered) != 1 || len(stale) != 1 {
			t.Errorf("filtered=%d stale=%d, want 1/1", len(filtered), len(stale))
		}
	})

	t.Run("message mismatch does not suppress", func(t *testing.T) {
		baseline := &Baseline{Entries: []BaselineEntry{
			{RuleID: "LL012", File: "marks/x.go", Line: 5, Message: "different"},
		}}
		filtered, stale := SuppressByBaseline(root, []*diag.Diagnostic{mkDiag(5, "m")}, baseline)
		if len(filtered) != 1 || len(stale) != 1 {
			t.Errorf("filtered=%d stale=%d, want 1/1", len(filtered), len(stale))
		}
	})

	t.Run("empty root leaves paths unchanged (legacy behavior)", func(t *testing.T) {
		baseline := &Baseline{Entries: []BaselineEntry{
			{RuleID: "LL012", File: absFile, Line: 5, Message: "m"},
		}}
		filtered, stale := SuppressByBaseline("", []*diag.Diagnostic{mkDiag(5, "m")}, baseline)
		if len(filtered) != 0 || len(stale) != 0 {
			t.Errorf("filtered=%d stale=%d, want 0/0 with empty root exact match", len(filtered), len(stale))
		}
	})
}
