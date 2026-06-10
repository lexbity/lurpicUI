package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/capindex"
	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/loader"
	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/rules"
)

// repoRoot walks up from the test file to find go.mod.
func e2eRepoRoot(tb testing.TB) string {
	tb.Helper()
	dir, err := os.Getwd()
	if err != nil {
		tb.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			tb.Fatal("go.mod not found")
		}
		dir = parent
	}
}

// e2eGoldenFixture mirrors the golden JSON structure for E2E tests.
type e2eGoldenFixture struct {
	RuleID       string `json:"rule_id"`
	Severity     string `json:"severity"`
	File         string `json:"file"`
	Line         int    `json:"line"`
	Message      string `json:"message"`
	RelatedLines []int  `json:"related_lines,omitempty"`
}

func TestE2E_StudioBaseline(t *testing.T) {
	root := e2eRepoRoot(t)
	studioDir := filepath.Join(root, "demos", "lurpic_studio")

	// Load the studio packages.
	result, err := loader.Load([]string{studioDir + "/..."}, loader.Config{})
	if err != nil {
		t.Fatal(err)
	}

	// Build capindex for LL004.
	capResult, capErr := loader.Load([]string{
		root + "/marks/...",
		root + "/layout/...",
		root + "/facet",
	}, loader.Config{})
	if capErr != nil {
		t.Fatal(capErr)
	}
	caps := capindex.Scan(capResult, capindex.ScanConfig{
		ModulePath: "codeburg.org/lexbit/lurpicui",
		ModuleRoot: root,
	})

	// Run all rules.
	ctx := &rules.Context{
		Files: result.Files,
		Pkgs:  result.Packages,
		Fset:  result.Fset,
		Index: caps,
	}
	diags := rules.Run(ctx, rules.DefaultRegistry, rules.RunConfig{})

	// Load golden file.
	goldenPath := filepath.Join(root, "cmd", "lurpiclint", "testdata", "e2e", "studio-golden.json")
	goldenData, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatal(err)
	}
	var expected []e2eGoldenFixture
	if err := json.Unmarshal(goldenData, &expected); err != nil {
		t.Fatal(err)
	}

	// Normalize diagnostics for comparison.
	if len(diags) != len(expected) {
		t.Fatalf("diagnostic count: got %d, want %d", len(diags), len(expected))
	}

	// Compare each diagnostic.
	for i, d := range diags {
		relFile := relStudioPath(d.Pos.Filename)
		related := make([]int, len(d.Related))
		for j, r := range d.Related {
			related[j] = r.Line
		}
		got := e2eGoldenFixture{
			RuleID:       d.RuleID,
			Severity:     d.Severity.String(),
			File:         relFile,
			Line:         d.Pos.Line,
			Message:      d.Message,
			RelatedLines: related,
		}
		want := expected[i]
		if got.RuleID != want.RuleID {
			t.Errorf("diagnostic %d: RuleID = %q, want %q", i, got.RuleID, want.RuleID)
		}
		if got.Severity != want.Severity {
			t.Errorf("diagnostic %d: Severity = %q, want %q", i, got.Severity, want.Severity)
		}
		// Normalize file path — both should be relative to studio/.
		if got.File != want.File && !filepath.IsAbs(want.File) {
			t.Errorf("diagnostic %d: File = %q, want %q", i, got.File, want.File)
		}
		if got.Line != want.Line {
			t.Errorf("diagnostic %d: Line = %d, want %d", i, got.Line, want.Line)
		}
		if got.Message != want.Message {
			t.Errorf("diagnostic %d:\n  Message: %q\n  want:    %q", i, got.Message, want.Message)
		}
		if len(got.RelatedLines) != len(want.RelatedLines) {
			t.Errorf("diagnostic %d: related_lines count = %d, want %d", i, len(got.RelatedLines), len(want.RelatedLines))
		} else {
			for j := range got.RelatedLines {
				if got.RelatedLines[j] != want.RelatedLines[j] {
					t.Errorf("diagnostic %d: related_lines[%d] = %d, want %d", i, j, got.RelatedLines[j], want.RelatedLines[j])
				}
			}
		}
	}
}

func TestE2E_StudioPorted(t *testing.T) {
	// This test is skipped until the studio port is complete.
	// Once all hand-rolled LayoutRoles in the studio are replaced with
	// built-in marks, this test should pass with zero diagnostics.
	t.Skip("Studio port not yet complete — replace hand-rolled LayoutRoles with built-in marks")

	root := e2eRepoRoot(t)
	studioDir := filepath.Join(root, "demos", "lurpic_studio")

	result, err := loader.Load([]string{studioDir + "/..."}, loader.Config{})
	if err != nil {
		t.Fatal(err)
	}

	capResult, capErr := loader.Load([]string{
		root + "/marks/...",
		root + "/layout/...",
		root + "/facet",
	}, loader.Config{})
	if capErr != nil {
		t.Fatal(capErr)
	}
	caps := capindex.Scan(capResult, capindex.ScanConfig{
		ModulePath: "codeburg.org/lexbit/lurpicui",
		ModuleRoot: root,
	})

	ctx := &rules.Context{
		Files: result.Files,
		Pkgs:  result.Packages,
		Fset:  result.Fset,
		Index: caps,
	}
	diags := rules.Run(ctx, rules.DefaultRegistry, rules.RunConfig{})

	if len(diags) > 0 {
		for _, d := range diags {
			t.Errorf("unexpected diagnostic in ported studio:\n  %s:%d: %s (%s)",
				relStudioPath(d.Pos.Filename), d.Pos.Line, d.Message, d.RuleID)
		}
	}
}

// relStudioPath converts an absolute file path to a path relative to the
// lurpic_studio/ directory.  E.g. ".../lurpic_studio/studio/root.go" →
// "studio/root.go".  Returns just the basename as fallback.
func relStudioPath(absPath string) string {
	const marker = "lurpic_studio/"
	idx := strings.Index(absPath, marker)
	if idx >= 0 {
		return absPath[idx+len(marker):]
	}
	return filepath.Base(absPath)
}
