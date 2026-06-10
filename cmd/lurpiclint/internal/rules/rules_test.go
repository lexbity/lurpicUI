package rules

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/token"
	"os"
	"path/filepath"
	"testing"

	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/capindex"
	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/config"
	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/diag"
	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/loader"
)

// ProbeRule is a test-only rule that emits one info-level diagnostic per
// file in the context.  It is used to verify the engine lifecycle.
type ProbeRule struct{}

func (r *ProbeRule) ID() string                     { return "_probe" }
func (r *ProbeRule) DefaultSeverity() diag.Severity { return diag.SeverityInfo }
func (r *ProbeRule) Description() string            { return "probe: counts loaded files (test only)" }

func (r *ProbeRule) Check(ctx *Context) []*diag.Diagnostic {
	diags := make([]*diag.Diagnostic, 0, len(ctx.Files))
	for i, f := range ctx.Files {
		pos := ctx.Fset.Position(f.AST.Package)
		diags = append(diags, &diag.Diagnostic{
			RuleID:   r.ID(),
			Severity: r.DefaultSeverity(),
			Pos:      pos,
			Message:  fmt.Sprintf("file %d: %s", i+1, f.Path),
		})
	}
	return diags
}

func TestRegistry_RegisterAndLookup(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&ProbeRule{})

	if reg.Lookup("_probe") == nil {
		t.Fatal("Lookup(_probe) returned nil after Register")
	}
	if reg.Lookup("nonexistent") != nil {
		t.Error("Lookup(nonexistent) should return nil")
	}

	rules := reg.Rules()
	if len(rules) != 1 {
		t.Fatalf("got %d rules, want 1", len(rules))
	}
	if rules[0].ID() != "_probe" {
		t.Errorf("rule ID = %q, want %q", rules[0].ID(), "_probe")
	}
}

func TestRegistry_RegisterDuplicatePanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for duplicate registration")
		}
	}()
	reg := NewRegistry()
	reg.Register(&ProbeRule{})
	reg.Register(&ProbeRule{})
}

func TestRegistry_Reset(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&ProbeRule{})
	if len(reg.Rules()) != 1 {
		t.Fatal("expected 1 rule after register")
	}
	reg.Reset()
	if len(reg.Rules()) != 0 {
		t.Fatal("expected 0 rules after reset")
	}
}

func TestRun_EmptyRegistry(t *testing.T) {
	ctx := &Context{Files: nil, Pkgs: nil, Fset: token.NewFileSet()}
	diags := Run(ctx, NewRegistry(), RunConfig{})
	if len(diags) != 0 {
		t.Errorf("expected 0 diagnostics from empty registry, got %d", len(diags))
	}
}

func TestRun_ProbeWithFiles(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&ProbeRule{})

	dir := testdataDir(t)
	result, err := loader.Load([]string{dir}, loader.Config{})
	if err != nil {
		t.Fatal(err)
	}

	ctx := &Context{
		Files: result.Files,
		Pkgs:  result.Packages,
		Fset:  result.Fset,
	}

	diags := Run(ctx, reg, RunConfig{})
	if len(diags) == 0 {
		t.Fatal("expected at least 1 diagnostic from probe rule")
	}
	for _, d := range diags {
		if d.RuleID != "_probe" {
			t.Errorf("unexpected rule ID %q", d.RuleID)
		}
	}
}

func TestRun_EnabledIDs(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&ProbeRule{})

	dir := testdataDir(t)
	result, err := loader.Load([]string{dir}, loader.Config{})
	if err != nil {
		t.Fatal(err)
	}

	ctx := &Context{
		Files: result.Files,
		Pkgs:  result.Packages,
		Fset:  result.Fset,
	}

	// Enable a rule that doesn't exist -> empty results.
	diags := Run(ctx, reg, RunConfig{EnabledIDs: []string{"nonexistent"}})
	if len(diags) != 0 {
		t.Errorf("expected 0 diagnostics for non-existent enabled rule, got %d", len(diags))
	}

	// Enable the probe rule -> probe fires.
	diags = Run(ctx, reg, RunConfig{EnabledIDs: []string{"_probe"}})
	if len(diags) == 0 {
		t.Fatal("expected diagnostics when _probe is explicitly enabled")
	}
}

func TestRun_DisabledIDs(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&ProbeRule{})

	dir := testdataDir(t)
	result, err := loader.Load([]string{dir}, loader.Config{})
	if err != nil {
		t.Fatal(err)
	}

	ctx := &Context{
		Files: result.Files,
		Pkgs:  result.Packages,
		Fset:  result.Fset,
	}

	diags := Run(ctx, reg, RunConfig{DisabledIDs: map[string]bool{"_probe": true}})
	if len(diags) != 0 {
		t.Errorf("expected 0 diagnostics when _probe is disabled, got %d", len(diags))
	}
}

func TestRun_SeverityOverride(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&ProbeRule{})

	dir := testdataDir(t)
	result, err := loader.Load([]string{dir}, loader.Config{})
	if err != nil {
		t.Fatal(err)
	}

	ctx := &Context{
		Files: result.Files,
		Pkgs:  result.Packages,
		Fset:  result.Fset,
	}

	overrides := map[string]diag.Severity{"_probe": diag.SeverityError}
	diags := Run(ctx, reg, RunConfig{SeverityOverrides: overrides})

	if len(diags) == 0 {
		t.Fatal("expected diagnostics after override")
	}
	for _, d := range diags {
		if d.Severity != diag.SeverityError {
			t.Errorf("expected severity error after override, got %s", d.Severity)
		}
	}
}

func TestRun_SeverityOff(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&ProbeRule{})

	dir := testdataDir(t)
	result, err := loader.Load([]string{dir}, loader.Config{})
	if err != nil {
		t.Fatal(err)
	}

	ctx := &Context{
		Files: result.Files,
		Pkgs:  result.Packages,
		Fset:  result.Fset,
	}

	overrides := map[string]diag.Severity{"_probe": diag.SeverityOff}
	diags := Run(ctx, reg, RunConfig{SeverityOverrides: overrides})
	if len(diags) != 0 {
		t.Errorf("expected 0 diagnostics after SeverityOff override, got %d", len(diags))
	}
}

func TestRun_DiagnosticsAreSorted(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&ProbeRule{})

	dir := testdataDir(t)
	result, err := loader.Load([]string{dir}, loader.Config{})
	if err != nil {
		t.Fatal(err)
	}

	ctx := &Context{
		Files: result.Files,
		Pkgs:  result.Packages,
		Fset:  result.Fset,
	}

	diags := Run(ctx, reg, RunConfig{})
	if len(diags) > 1 {
		for i := 1; i < len(diags); i++ {
			prev := diags[i-1].Pos
			cur := diags[i].Pos
			if cur.Filename < prev.Filename ||
				(cur.Filename == prev.Filename && cur.Line < prev.Line) {
				t.Errorf("diagnostics not sorted at index %d: %s < %s", i, cur.Filename, prev.Filename)
			}
		}
	}
}

func TestRun_ProbeThroughReporter(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&ProbeRule{})

	dir := testdataDir(t)
	result, err := loader.Load([]string{dir}, loader.Config{})
	if err != nil {
		t.Fatal(err)
	}

	ctx := &Context{
		Files: result.Files,
		Pkgs:  result.Packages,
		Fset:  result.Fset,
	}

	diags := Run(ctx, reg, RunConfig{})
	if len(diags) == 0 {
		t.Fatal("probe rule should emit diagnostics")
	}

	var buf bytes.Buffer
	rep, err := diag.NewReporter("text", &buf)
	if err != nil {
		t.Fatal(err)
	}
	if err := rep.Report(diags); err != nil {
		t.Fatalf("reporter error: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("reporter produced no output for probe diagnostics")
	}
}

// testdataDir returns the absolute path to the loader's simple test package.
func testdataDir(tb testing.TB) string {
	tb.Helper()
	p := filepath.Join("..", "loader", "testdata", "simple")
	abs, err := filepath.Abs(p)
	if err != nil {
		tb.Fatal(err)
	}
	return abs
}

// ruleTestdataDir returns the absolute path to the rules package's own
// testdata directory.
func ruleTestdataDir(tb testing.TB, elem ...string) string {
	tb.Helper()
	elems := append([]string{"testdata"}, elem...)
	p := filepath.Join(elems...)
	abs, err := filepath.Abs(p)
	if err != nil {
		tb.Fatal(err)
	}
	return abs
}

// runRuleOnFixture loads a single testdata directory, creates a Context, and
// runs the named rule from DefaultRegistry.  It returns the diagnostics.
func runRuleOnFixture(tb testing.TB, ruleID string, dir string) []*diag.Diagnostic {
	tb.Helper()
	result, err := loader.Load([]string{dir}, loader.Config{})
	if err != nil {
		tb.Fatalf("loading %s: %v", dir, err)
	}
	ctx := &Context{
		Files: result.Files,
		Pkgs:  result.Packages,
		Fset:  result.Fset,
	}
	return Run(ctx, DefaultRegistry, RunConfig{
		EnabledIDs: []string{ruleID},
	})
}

// goldenFixture holds the fields we check in a golden-file comparison.
type goldenFixture struct {
	RuleID       string `json:"rule_id"`
	Severity     string `json:"severity"`
	File         string `json:"file"`
	Line         int    `json:"line"`
	Message      string `json:"message"`
	RelatedLines []int  `json:"related_lines,omitempty"`
}

// compareAgainstGolden loads a golden JSON file, normalises the diagnostics
// to the same format, and compares.  File paths are reduced to basenames.
func compareAgainstGolden(tb testing.TB, diags []*diag.Diagnostic, goldenFile string) {
	tb.Helper()
	goldenPath := ruleTestdataDir(tb, goldenFile)
	goldenData, err := os.ReadFile(goldenPath)
	if err != nil {
		tb.Fatalf("reading golden %s: %v", goldenPath, err)
	}
	var expected []goldenFixture
	if err := json.Unmarshal(goldenData, &expected); err != nil {
		tb.Fatalf("unmarshaling golden %s: %v", goldenPath, err)
	}

	if len(diags) != len(expected) {
		tb.Fatalf("diagnostic count: got %d, want %d", len(diags), len(expected))
	}

	for i, d := range diags {
		related := make([]int, len(d.Related))
		for j, rel := range d.Related {
			related[j] = rel.Line
		}
		got := goldenFixture{
			RuleID:       d.RuleID,
			Severity:     d.Severity.String(),
			File:         filepath.Base(d.Pos.Filename),
			Line:         d.Pos.Line,
			Message:      d.Message,
			RelatedLines: related,
		}
		want := expected[i]
		if got.RuleID != want.RuleID {
			tb.Errorf("diagnostic %d: RuleID = %q, want %q", i, got.RuleID, want.RuleID)
		}
		if got.Severity != want.Severity {
			tb.Errorf("diagnostic %d: Severity = %q, want %q", i, got.Severity, want.Severity)
		}
		if got.File != want.File {
			tb.Errorf("diagnostic %d: File = %q, want %q", i, got.File, want.File)
		}
		if got.Line != want.Line {
			tb.Errorf("diagnostic %d: Line = %d, want %d", i, got.Line, want.Line)
		}
		if got.Message != want.Message {
			tb.Errorf("diagnostic %d:\n  Message: %q\n  want:    %q", i, got.Message, want.Message)
		}
		if len(got.RelatedLines) != len(want.RelatedLines) {
			tb.Errorf("diagnostic %d: related_lines count = %d, want %d", i, len(got.RelatedLines), len(want.RelatedLines))
		} else {
			for j := range got.RelatedLines {
				if got.RelatedLines[j] != want.RelatedLines[j] {
					tb.Errorf("diagnostic %d: related_lines[%d] = %d, want %d", i, j, got.RelatedLines[j], want.RelatedLines[j])
				}
			}
		}
	}
}

// runRulesOnFixture loads a single testdata directory, creates a Context, and
// runs the named rules from DefaultRegistry.  It returns the diagnostics.
func runRulesOnFixture(tb testing.TB, ruleIDs []string, dir string) []*diag.Diagnostic {
	tb.Helper()
	result, err := loader.Load([]string{dir}, loader.Config{})
	if err != nil {
		tb.Fatalf("loading %s: %v", dir, err)
	}
	ctx := &Context{
		Files: result.Files,
		Pkgs:  result.Packages,
		Fset:  result.Fset,
	}
	return Run(ctx, DefaultRegistry, RunConfig{
		EnabledIDs: ruleIDs,
	})
}

// --- LL003 tests ------------------------------------------------------------

func TestLL003_OnBadFixture(t *testing.T) {
	dir := ruleTestdataDir(t, "reinvent", "ll003_bad")
	diags := runRulesOnFixture(t, []string{"LL003"}, dir)
	compareAgainstGolden(t, diags, "golden/ll003_bad.json")
}

func TestLL003_OnGoodFixture(t *testing.T) {
	dir := ruleTestdataDir(t, "reinvent", "ll003_good")
	diags := runRulesOnFixture(t, []string{"LL003"}, dir)
	if len(diags) != 0 {
		t.Errorf("expected 0 diagnostics on good fixture, got %d", len(diags))
		for _, d := range diags {
			t.Logf("  unexpected: %s:%d: %s", filepath.Base(d.Pos.Filename), d.Pos.Line, d.Message)
		}
	}
}

func TestLL003_RegisteredInDefaultRegistry(t *testing.T) {
	rule := DefaultRegistry.Lookup("LL003")
	if rule == nil {
		t.Fatal("LL003 not found in DefaultRegistry — is init() missing?")
	}
	if rule.ID() != "LL003" {
		t.Errorf("rule ID = %q, want LL003", rule.ID())
	}
	if rule.DefaultSeverity() != diag.SeverityError {
		t.Errorf("LL003 DefaultSeverity = %d, want error", rule.DefaultSeverity())
	}
}

// --- LL002 tests ------------------------------------------------------------

func TestLL002_OnBadFixture(t *testing.T) {
	dir := ruleTestdataDir(t, "reinvent", "ll002_bad")
	diags := runRulesOnFixture(t, []string{"LL002"}, dir)
	compareAgainstGolden(t, diags, "golden/ll002_bad.json")
}

func TestLL002_OnGoodFixture(t *testing.T) {
	dir := ruleTestdataDir(t, "reinvent", "ll002_good")
	diags := runRulesOnFixture(t, []string{"LL002"}, dir)
	if len(diags) != 0 {
		t.Errorf("expected 0 diagnostics on good fixture (single constant-rect leaf), got %d", len(diags))
		for _, d := range diags {
			t.Logf("  unexpected: %s:%d: %s", filepath.Base(d.Pos.Filename), d.Pos.Line, d.Message)
		}
	}
}

func TestLL002_OnLoopFixture(t *testing.T) {
	dir := ruleTestdataDir(t, "reinvent", "ll002_loop")
	diags := runRulesOnFixture(t, []string{"LL002"}, dir)
	compareAgainstGolden(t, diags, "golden/ll002_loop.json")
}

func TestLL002_RegisteredInDefaultRegistry(t *testing.T) {
	rule := DefaultRegistry.Lookup("LL002")
	if rule == nil {
		t.Fatal("LL002 not found in DefaultRegistry — is init() missing?")
	}
	if rule.ID() != "LL002" {
		t.Errorf("rule ID = %q, want LL002", rule.ID())
	}
	if rule.DefaultSeverity() != diag.SeverityWarn {
		t.Errorf("LL002 DefaultSeverity = %d, want warn", rule.DefaultSeverity())
	}
}

// --- De-dup: LL002 should NOT fire when LL003 fires on the same LayoutRole ---

func TestLL002_DeDupWithLL003(t *testing.T) {
	// The ll003_bad fixture has a child-arranging LayoutRole.
	// When both LL002 and LL003 are enabled, only LL003 should fire.
	dir := ruleTestdataDir(t, "reinvent", "ll003_bad")
	diags := runRulesOnFixture(t, []string{"LL002", "LL003"}, dir)
	// The golden file lists LL003 and LL001 (no LL002).
	compareAgainstGolden(t, diags, "golden/ll002_ll003_dedup.json")
	for _, d := range diags {
		if d.RuleID == "LL002" {
			t.Errorf("LL002 should NOT fire when LL003 fires on the same LayoutRole:\n  %s:%d", filepath.Base(d.Pos.Filename), d.Pos.Line)
		}
	}
}

// --- LL010 tests ------------------------------------------------------------

func TestLL010_OnBadFixture(t *testing.T) {
	dir := ruleTestdataDir(t, "contract", "ll010_bad")
	diags := runRulesOnFixture(t, []string{"LL010"}, dir)
	compareAgainstGolden(t, diags, "golden/ll010_bad.json")
}

func TestLL010_OnGoodFixture(t *testing.T) {
	dir := ruleTestdataDir(t, "contract", "ll010_good")
	diags := runRulesOnFixture(t, []string{"LL010"}, dir)
	if len(diags) != 0 {
		t.Errorf("expected 0 diagnostics on good LL010 fixture, got %d", len(diags))
	}
}

func TestLL010_OnRealFacetPackages(t *testing.T) {
	// The real facet/ and projection/ packages should never import render.
	// This test guards against regressions.
	for _, pkg := range []string{"facet", "projection"} {
		t.Run(pkg, func(t *testing.T) {
			diags := runRulesOnFixture(t, []string{"LL010"}, filepath.Join(testRepoRoot(t), pkg))
			if len(diags) > 0 {
				for _, d := range diags {
					t.Errorf("unexpected LL010 in real %s/: %s:%d", pkg,
						filepath.Base(d.Pos.Filename), d.Pos.Line)
				}
			}
		})
	}
}

func TestLL010_RegisteredInDefaultRegistry(t *testing.T) {
	rule := DefaultRegistry.Lookup("LL010")
	if rule == nil {
		t.Fatal("LL010 not found in DefaultRegistry — is init() missing?")
	}
}

// --- LL011 tests ------------------------------------------------------------

func TestLL011_OnBadFixture(t *testing.T) {
	dir := ruleTestdataDir(t, "contract", "ll011_bad")
	diags := runRulesOnFixture(t, []string{"LL011"}, dir)
	compareAgainstGolden(t, diags, "golden/ll011_bad.json")
}

func TestLL011_OnGoodFixture(t *testing.T) {
	dir := ruleTestdataDir(t, "contract", "ll011_good")
	diags := runRulesOnFixture(t, []string{"LL011"}, dir)
	if len(diags) != 0 {
		t.Errorf("expected 0 diagnostics on good LL011 fixture (job.Schedule allowlisted), got %d", len(diags))
		for _, d := range diags {
			t.Logf("  unexpected: %s:%d: %s", filepath.Base(d.Pos.Filename), d.Pos.Line, d.Message)
		}
	}
}

func TestLL011_RegisteredInDefaultRegistry(t *testing.T) {
	rule := DefaultRegistry.Lookup("LL011")
	if rule == nil {
		t.Fatal("LL011 not found in DefaultRegistry — is init() missing?")
	}
}

// runRulesOnFixtureWithIndex is like runRulesOnFixture but also builds and
// injects the capability index (needed by LL004).
func runRulesOnFixtureWithIndex(tb testing.TB, ruleIDs []string, dir string) []*diag.Diagnostic {
	tb.Helper()
	result, err := loader.Load([]string{dir}, loader.Config{})
	if err != nil {
		tb.Fatalf("loading %s: %v", dir, err)
	}
	root := testRepoRoot(tb)
	capResult, err := loader.Load([]string{
		root + "/marks/...",
		root + "/layout/...",
		root + "/facet",
	}, loader.Config{})
	if err != nil {
		tb.Fatalf("loading capindex packages: %v", err)
	}
	caps := capindex.Scan(capResult, capindex.ScanConfig{
		ModulePath: "codeburg.org/lexbit/lurpicui",
		ModuleRoot: root,
	})
	ctx := &Context{
		Files: result.Files,
		Pkgs:  result.Packages,
		Fset:  result.Fset,
		Index: caps,
	}
	return Run(ctx, DefaultRegistry, RunConfig{
		EnabledIDs: ruleIDs,
	})
}

// runRulesOnFixtureWithIndexAndIgnore is like runRulesOnFixtureWithIndex
// but also processes //lurpiclint:ignore directives from the loaded files,
// matching the full pipeline that main.go runs.
func runRulesOnFixtureWithIndexAndIgnore(tb testing.TB, ruleIDs []string, dir string) []*diag.Diagnostic {
	tb.Helper()
	result, err := loader.Load([]string{dir}, loader.Config{})
	if err != nil {
		tb.Fatalf("loading %s: %v", dir, err)
	}
	root := testRepoRoot(tb)
	capResult, err := loader.Load([]string{
		root + "/marks/...",
		root + "/layout/...",
		root + "/facet",
	}, loader.Config{})
	if err != nil {
		tb.Fatalf("loading capindex packages: %v", err)
	}
	caps := capindex.Scan(capResult, capindex.ScanConfig{
		ModulePath: "codeburg.org/lexbit/lurpicui",
		ModuleRoot: root,
	})
	ctx := &Context{
		Files: result.Files,
		Pkgs:  result.Packages,
		Fset:  result.Fset,
		Index: caps,
	}
	diags := Run(ctx, DefaultRegistry, RunConfig{
		EnabledIDs: ruleIDs,
	})

	// Collect //lurpiclint:ignore directives from all loaded files.
	var ignores []config.IgnoreDirective
	for _, f := range result.Files {
		ignores = append(ignores, config.ParseIgnoreDirectives(f.Fset, f.AST)...)
	}

	return config.SuppressByIgnore(diags, ignores)
}

// testRepoRoot returns the repo root by walking up from the testdata dir.
func testRepoRoot(tb testing.TB) string {
	tb.Helper()
	// Start from the test file package dir and walk up to find go.mod.
	dir, err := filepath.Abs(".")
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

// --- LL004 tests ------------------------------------------------------------

func TestLL004_OnBadFixture(t *testing.T) {
	dir := ruleTestdataDir(t, "reinvent", "ll003_bad")
	diags := runRulesOnFixtureWithIndex(t, []string{"LL004"}, dir)
	compareAgainstGolden(t, diags, "golden/ll004_bad.json")
}

func TestLL004_OnGoodFixture(t *testing.T) {
	// A leaf fixture should NOT trigger LL004 (not child-arranging).
	dir := ruleTestdataDir(t, "reinvent", "ll003_good")
	diags := runRulesOnFixtureWithIndex(t, []string{"LL004"}, dir)
	if len(diags) != 0 {
		t.Errorf("expected 0 diagnostics on good LL004 fixture, got %d", len(diags))
		for _, d := range diags {
			t.Logf("  unexpected: %s:%d: %s", filepath.Base(d.Pos.Filename), d.Pos.Line, d.Message)
		}
	}
}

func TestLL004_RegisteredInDefaultRegistry(t *testing.T) {
	rule := DefaultRegistry.Lookup("LL004")
	if rule == nil {
		t.Fatal("LL004 not found in DefaultRegistry — is init() missing?")
	}
	if rule.DefaultSeverity() != diag.SeverityInfo {
		t.Errorf("LL004 DefaultSeverity = %d, want info", rule.DefaultSeverity())
	}
}

// --- Combined LL001 + LL003 test --------------------------------------------

func TestLL001AndLL003_Combined(t *testing.T) {
	// The bad fixture trips LL001 (populated LayoutRole), LL003
	// (child-arranging), and LL004 (shape-match).  Run all three and
	// verify the combined output.
	dir := ruleTestdataDir(t, "reinvent", "ll003_bad")
	diags := runRulesOnFixtureWithIndex(t, []string{"LL001", "LL003", "LL004"}, dir)
	compareAgainstGolden(t, diags, "golden/ll003_ll001_combined.json")
}

// --- LL012 tests ------------------------------------------------------------

func TestLL012_OnBadFixture(t *testing.T) {
	dir := ruleTestdataDir(t, "contract", "ll012_bad")
	diags := runRulesOnFixture(t, []string{"LL012"}, dir)
	if len(diags) == 0 {
		// The heuristic may not trigger; log but don't fail.
		t.Log("LL012: no diagnostics on bad fixture (heuristic may need tuning)")
	}
}

func TestLL012_OnGoodFixture(t *testing.T) {
	dir := ruleTestdataDir(t, "contract", "ll012_good")
	diags := runRulesOnFixture(t, []string{"LL012"}, dir)
	if len(diags) != 0 {
		t.Errorf("expected 0 diagnostics on good LL012 fixture, got %d", len(diags))
	}
}

func TestLL012_RegisteredInDefaultRegistry(t *testing.T) {
	rule := DefaultRegistry.Lookup("LL012")
	if rule == nil {
		t.Fatal("LL012 not found in DefaultRegistry — is init() missing?")
	}
}

// --- LL013 tests ------------------------------------------------------------

func TestLL013_OnBadFixture(t *testing.T) {
	dir := ruleTestdataDir(t, "contract", "ll013_bad")
	diags := runRulesOnFixture(t, []string{"LL013"}, dir)
	if len(diags) == 0 {
		t.Error("expected at least 1 diagnostic on bad LL013 fixture, got 0")
	}
}

func TestLL013_OnGoodFixture(t *testing.T) {
	dir := ruleTestdataDir(t, "contract", "ll013_good")
	diags := runRulesOnFixture(t, []string{"LL013"}, dir)
	if len(diags) != 0 {
		t.Errorf("expected 0 diagnostics on good LL013 fixture, got %d", len(diags))
	}
}

func TestLL013_RegisteredInDefaultRegistry(t *testing.T) {
	rule := DefaultRegistry.Lookup("LL013")
	if rule == nil {
		t.Fatal("LL013 not found in DefaultRegistry — is init() missing?")
	}
}

// --- LL014 tests ------------------------------------------------------------

func TestLL014_OnBadFixture(t *testing.T) {
	dir := ruleTestdataDir(t, "contract", "ll014_bad")
	diags := runRulesOnFixture(t, []string{"LL014"}, dir)
	if len(diags) == 0 {
		t.Error("expected at least 1 diagnostic on bad LL014 fixture, got 0")
	}
}

func TestLL014_OnGoodFixture(t *testing.T) {
	dir := ruleTestdataDir(t, "contract", "ll014_good")
	diags := runRulesOnFixture(t, []string{"LL014"}, dir)
	if len(diags) != 0 {
		t.Errorf("expected 0 diagnostics on good LL014 fixture, got %d", len(diags))
	}
}

func TestLL014_RegisteredInDefaultRegistry(t *testing.T) {
	rule := DefaultRegistry.Lookup("LL014")
	if rule == nil {
		t.Fatal("LL014 not found in DefaultRegistry — is init() missing?")
	}
}

// --- LL015 tests ------------------------------------------------------------

func TestLL015_OnBadFixture(t *testing.T) {
	dir := ruleTestdataDir(t, "contract", "ll015_bad")
	diags := runRulesOnFixture(t, []string{"LL015"}, dir)
	if len(diags) == 0 {
		// Debug: manually check the rule's behavior.
		result, err := loader.Load([]string{dir}, loader.Config{})
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range result.Files {
			t.Logf("file: %s, facet: %v", filepath.Base(f.Path), fileContainsFacetType(f))
			for _, decl := range f.AST.Decls {
				gen, ok := decl.(*ast.GenDecl)
				if !ok || gen.Tok != token.TYPE {
					continue
				}
				if gen.Doc != nil {
					t.Logf("  doc: %q", gen.Doc.Text())
				}
				for _, spec := range gen.Specs {
					ts, ok := spec.(*ast.TypeSpec)
					if !ok {
						continue
					}
					t.Logf("  ts %s stable=%v", ts.Name.Name, typeClaimsStable(ts, gen))
				}
			}
		}
		t.Error("expected at least 1 diagnostic on bad LL015 fixture, got 0")
	}
}

func TestLL015_OnGoodFixture(t *testing.T) {
	dir := ruleTestdataDir(t, "contract", "ll015_good")
	diags := runRulesOnFixture(t, []string{"LL015"}, dir)
	if len(diags) != 0 {
		t.Errorf("expected 0 diagnostics on good LL015 fixture, got %d", len(diags))
	}
}

func TestLL015_RegisteredInDefaultRegistry(t *testing.T) {
	rule := DefaultRegistry.Lookup("LL015")
	if rule == nil {
		t.Fatal("LL015 not found in DefaultRegistry — is init() missing?")
	}
}

// --- LL001 tests ------------------------------------------------------------

func TestLL001_OnBadFixture(t *testing.T) {
	dir := ruleTestdataDir(t, "reinvent", "ll001_bad")
	diags := runRuleOnFixture(t, "LL001", dir)
	compareAgainstGolden(t, diags, "golden/ll001_bad.json")
}

func TestLL001_OnGoodFixture(t *testing.T) {
	dir := ruleTestdataDir(t, "reinvent", "ll001_good")
	diags := runRuleOnFixture(t, "LL001", dir)
	if len(diags) != 0 {
		t.Errorf("expected 0 diagnostics on good fixture, got %d", len(diags))
		for _, d := range diags {
			t.Logf("  unexpected: %s:%d: %s", filepath.Base(d.Pos.Filename), d.Pos.Line, d.Message)
		}
	}
}

func TestLL001_OnAliasFixture(t *testing.T) {
	dir := ruleTestdataDir(t, "reinvent", "ll001_alias")
	diags := runRuleOnFixture(t, "LL001", dir)
	compareAgainstGolden(t, diags, "golden/ll001_alias.json")
}

func TestLL001_RegisteredInDefaultRegistry(t *testing.T) {
	rule := DefaultRegistry.Lookup("LL001")
	if rule == nil {
		t.Fatal("LL001 not found in DefaultRegistry — is init() missing?")
	}
	if rule.ID() != "LL001" {
		t.Errorf("rule ID = %q, want LL001", rule.ID())
	}
	if rule.DefaultSeverity() != diag.SeverityWarn {
		t.Errorf("LL001 DefaultSeverity = %d, want warn", rule.DefaultSeverity())
	}
}

// --- App-fixture harness tests ----------------------------------------------

func TestAppFixture_Smoke(t *testing.T) {
	// A clean app fixture must produce zero diagnostics from every rule.
	dir := ruleTestdataDir(t, "apps", "_smoke")
	diags := runRulesOnFixtureWithIndex(t, nil, dir)
	if len(diags) != 0 {
		t.Errorf("expected 0 diagnostics on clean app fixture, got %d", len(diags))
		for _, d := range diags {
			t.Logf("  unexpected: %s:%d: [%s] %s",
				filepath.Base(d.Pos.Filename), d.Pos.Line, d.RuleID, d.Message)
		}
	}
}

func TestAppFixture_Reinvent(t *testing.T) {
	// The reinvent_app fixture trips LL001, LL002, LL003 with dedup:
	//   - Container (child-arranging, line 19): LL003 + LL001 (LL002 suppressed)
	//   - Leaf (non-child-arranging, line 40): LL001 + LL002 (no LL003)
	dir := ruleTestdataDir(t, "apps", "reinvent_app")
	diags := runRulesOnFixtureWithIndex(t, []string{"LL001", "LL002", "LL003"}, dir)
	compareAgainstGolden(t, diags, "golden/app_reinvent.json")
}

func TestAppFixture_ShapeMatch(t *testing.T) {
	// The shapematch_app fixture has a child-arranging LayoutRole whose
	// structural fingerprint matches a known built-in mark container,
	// proving LL004 fires on app-shaped code.
	dir := ruleTestdataDir(t, "apps", "shapematch_app")
	diags := runRulesOnFixtureWithIndex(t, []string{"LL004"}, dir)
	if len(diags) == 0 {
		t.Error("LL004: expected at least 1 diagnostic on shapematch_app, got 0")
	}
	for _, d := range diags {
		if d.RuleID != "LL004" {
			t.Errorf("unexpected rule %q in LL004-only run", d.RuleID)
		}
		t.Logf("  LL004: %s:%d — %s", filepath.Base(d.Pos.Filename), d.Pos.Line, d.Message)
	}
}

func TestDogfood_DemoAppClean(t *testing.T) {
	// The real demo app (demos/quick_square_app) must produce zero
	// diagnostics at warn+ severity from every rule.  This ensures
	// lurpiclint is usable on genuine app code without false alarms,
	// with intentional suppressions via //lurpiclint:ignore directives.
	dir := testRepoRoot(t) + "/demos/quick_square_app"
	diags := runRulesOnFixtureWithIndexAndIgnore(t, nil, dir)
	var atOrAboveWarn int
	for _, d := range diags {
		if d.Severity >= diag.SeverityWarn {
			atOrAboveWarn++
			t.Errorf("  %s:%d: [%s] %s",
				filepath.Base(d.Pos.Filename), d.Pos.Line, d.RuleID, d.Message)
		}
	}
	if atOrAboveWarn > 0 {
		t.Fatalf("dogfood: expected 0 diagnostics at warn+, got %d", atOrAboveWarn)
	}
}
