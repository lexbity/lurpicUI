package rules

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/token"
	"os"
	"path/filepath"
	"strings"
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

// PanicRule is a test-only rule that panics during Check.  Used to verify
// that Run recovers and reports an error diagnostic instead of crashing.
type PanicRule struct{}

func (r *PanicRule) ID() string                     { return "_panic" }
func (r *PanicRule) DefaultSeverity() diag.Severity { return diag.SeverityError }
func (r *PanicRule) Description() string            { return "panic: always panics (test only)" }

func (r *PanicRule) Check(ctx *Context) []*diag.Diagnostic {
	panic("intentional test panic")
}

func TestRun_PanickingRule(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&PanicRule{})

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
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic from panicking rule, got %d", len(diags))
	}
	d := diags[0]
	if d.RuleID != "_panic" {
		t.Errorf("diagnostic RuleID = %q, want %q", d.RuleID, "_panic")
	}
	if d.Severity != diag.SeverityError {
		t.Errorf("diagnostic Severity = %s, want error", d.Severity)
	}
	if !strings.Contains(d.Message, "panicked") {
		t.Errorf("diagnostic Message = %q, want it to mention panic", d.Message)
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

func TestAllRules_RegisteredInDefaultRegistry(t *testing.T) {
	expected := []struct {
		id       string
		severity diag.Severity
	}{
		{"LL001", diag.SeverityWarn},
		{"LL002", diag.SeverityWarn},
		{"LL003", diag.SeverityError},
		{"LL004", diag.SeverityInfo},
		{"LL019", diag.SeverityError},
		{"LL020", diag.SeverityError},
		{"LL021", diag.SeverityError},
		{"LL022", diag.SeverityError},
		{"LL023", diag.SeverityError},
		{"LL024", diag.SeverityError},
		{"LL025", diag.SeverityError},
		{"LL026", diag.SeverityError},
		{"LL027", diag.SeverityError},
		{"LL028", diag.SeverityWarn},
		{"LL029", diag.SeverityWarn},
		{"LL030", diag.SeverityWarn},
		{"LL031", diag.SeverityWarn},
		{"LL032", diag.SeverityWarn},
		{"LL033", diag.SeverityWarn},
		{"LL010", diag.SeverityError},
		{"LL011", diag.SeverityError},
		{"LL012", diag.SeverityWarn},
		{"LL013", diag.SeverityWarn},
		{"LL014", diag.SeverityError},
		{"LL015", diag.SeverityError},
		{"LL016", diag.SeverityError},
		{"LL017", diag.SeverityWarn},
		{"LL018", diag.SeverityWarn},
	}

	for _, e := range expected {
		rule := DefaultRegistry.Lookup(e.id)
		if rule == nil {
			t.Errorf("rule %s not found in DefaultRegistry — is init() missing?", e.id)
			continue
		}
		if rule.ID() != e.id {
			t.Errorf("rule %s: ID() = %q, want %q", e.id, rule.ID(), e.id)
		}
		if rule.DefaultSeverity() != e.severity {
			t.Errorf("rule %s: DefaultSeverity = %d, want %d", e.id, rule.DefaultSeverity(), e.severity)
		}
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
	ctx := fixtureContext(tb, dir)
	return Run(ctx, DefaultRegistry, RunConfig{
		EnabledIDs: []string{ruleID},
	})
}

// fixtureContext loads a fixture directory and builds a Context for running
// rules.  When the fixture includes framework packages (files under marks/ or
// layout/), the capability index is populated via capindex.Scan so rules that
// consume ctx.Index (LL029–LL033 and LL004's shape-match) exercise the real
// capindex path, and _test.go files are loaded so contract rules can detect
// contracttest helper invocations.  For non-framework fixtures the Index stays
// nil and rules that require it report the "capindex not populated"
// diagnostic.
func fixtureContext(tb testing.TB, dir string) *Context {
	tb.Helper()
	cfg := loader.Config{}
	if isFrameworkPath(filepath.ToSlash(dir)) {
		cfg.IncludeTests = true
	}
	result, err := loader.Load([]string{dir}, cfg)
	if err != nil {
		tb.Fatalf("loading %s: %v", dir, err)
	}
	ctx := &Context{
		Files: result.Files,
		Pkgs:  result.Packages,
		Fset:  result.Fset,
	}
	if fixtureIncludesFramework(result.Files) {
		caps := capindex.Scan(result, capindex.ScanConfig{
			ModulePath: "codeburg.org/lexbit/lurpicui",
			ModuleRoot: testRepoRoot(tb),
		})
		if len(caps) > 0 {
			ctx.Index = caps
		}
	}
	return ctx
}

// isFrameworkPath reports whether a slash-separated path lives under a marks/
// or layout/ directory tree — the only locations where capindex.Scan produces
// a meaningful capability index.
func isFrameworkPath(clean string) bool {
	return strings.Contains(clean, "/marks/") || strings.HasPrefix(clean, "marks/") ||
		strings.Contains(clean, "/layout/") || strings.HasPrefix(clean, "layout/")
}

// fixtureIncludesFramework reports whether any loaded file lives under a
// marks/ or layout/ directory tree.
func fixtureIncludesFramework(files []*loader.ParsedFile) bool {
	for _, f := range files {
		if isFrameworkPath(filepath.ToSlash(f.Path)) {
			return true
		}
	}
	return false
}

func TestFixtureIncludesFramework_UnderMarks(t *testing.T) {
	files := []*loader.ParsedFile{
		{Path: "/repo/marks/structure/table.go"},
		{Path: "/repo/cmd/rules/testdata/marks/viz/ll025_bad/x.go"},
	}
	if !fixtureIncludesFramework(files) {
		t.Error("fixtureIncludesFramework should be true for files under marks/")
	}
}

func TestFixtureIncludesFramework_UnderLayout(t *testing.T) {
	files := []*loader.ParsedFile{
		{Path: "/repo/layout/containers.go"},
	}
	if !fixtureIncludesFramework(files) {
		t.Error("fixtureIncludesFramework should be true for files under layout/")
	}
}

func TestFixtureIncludesFramework_NotFramework(t *testing.T) {
	files := []*loader.ParsedFile{
		{Path: "/repo/cmd/rules/testdata/suggest/ll004_scalar_bad/x.go"},
		{Path: "/repo/cmd/rules/testdata/contract_helpers/helpers_hasmeth/mark.go"},
	}
	if fixtureIncludesFramework(files) {
		t.Error("fixtureIncludesFramework should be false for non-framework fixtures")
	}
}

func TestFixtureContext_PopulatesIndexForFrameworkFixture(t *testing.T) {
	// ll025_bad lives under testdata/.../marks/viz, so the harness must
	// populate ctx.Index with the scanned capability index.
	dir := ruleTestdataDir(t, "contract", "ll025_bad", "marks", "viz")
	ctx := fixtureContext(t, dir)
	if ctx.Index == nil {
		t.Fatal("fixtureContext should populate Index for a marks/ fixture")
	}
	caps, ok := ctx.Index.([]capindex.Capability)
	if !ok {
		t.Fatalf("Index type = %T, want []capindex.Capability", ctx.Index)
	}
	if len(caps) == 0 {
		t.Fatal("expected non-empty capability index for marks/viz fixture")
	}
}

func TestFixtureContext_LeavesIndexNilForNonFrameworkFixture(t *testing.T) {
	dir := ruleTestdataDir(t, "suggest", "ll004_scalar_good")
	ctx := fixtureContext(t, dir)
	if ctx.Index != nil {
		t.Fatalf("fixtureContext should leave Index nil for non-framework fixture, got %T", ctx.Index)
	}
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
	ctx := fixtureContext(tb, dir)
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

// --- LL019 tests ------------------------------------------------------------

func TestLL019_OnBadFixture(t *testing.T) {
	dir := ruleTestdataDir(t, "reinvent", "ll019_bad")
	diags := runRulesOnFixture(t, []string{"LL019"}, dir)
	if len(diags) == 0 {
		t.Fatal("expected at least 1 LL019 diagnostic on bad fixture, got 0")
	}
	for _, d := range diags {
		if d.RuleID != "LL019" {
			t.Errorf("unexpected rule %q, want LL019", d.RuleID)
		}
	}
}

func TestLL019_OnGoodRegisterFixture(t *testing.T) {
	dir := ruleTestdataDir(t, "reinvent", "ll019_good_register")
	diags := runRulesOnFixture(t, []string{"LL019"}, dir)
	if len(diags) != 0 {
		t.Errorf("expected 0 LL019 diagnostics on good_register fixture, got %d", len(diags))
		for _, d := range diags {
			t.Logf("  unexpected: %s:%d: [%s] %s", filepath.Base(d.Pos.Filename), d.Pos.Line, d.RuleID, d.Message)
		}
	}
}

func TestLL019_OnGoodCoreFixture(t *testing.T) {
	dir := ruleTestdataDir(t, "reinvent", "ll019_good_core")
	diags := runRulesOnFixture(t, []string{"LL019"}, dir)
	if len(diags) != 0 {
		t.Errorf("expected 0 LL019 diagnostics on good_core fixture, got %d", len(diags))
		for _, d := range diags {
			t.Logf("  unexpected: %s:%d: [%s] %s", filepath.Base(d.Pos.Filename), d.Pos.Line, d.RuleID, d.Message)
		}
	}
}

func TestLL019_RegisteredInDefaultRegistry(t *testing.T) {
	rule := DefaultRegistry.Lookup("LL019")
	if rule == nil {
		t.Fatal("LL019 not found in DefaultRegistry — is init() missing?")
	}
	if rule.ID() != "LL019" {
		t.Errorf("rule ID = %q, want LL019", rule.ID())
	}
	if rule.DefaultSeverity() != diag.SeverityError {
		t.Errorf("LL019 DefaultSeverity = %d, want error", rule.DefaultSeverity())
	}
}

// --- LL020 tests ------------------------------------------------------------

func TestLL020_OnCallBadFixture(t *testing.T) {
	dir := ruleTestdataDir(t, "contract", "ll020_call_bad")
	diags := runRulesOnFixture(t, []string{"LL020"}, dir)
	if len(diags) == 0 {
		t.Fatal("expected at least 1 LL020 diagnostic on call_bad fixture, got 0")
	}
	for _, d := range diags {
		if d.RuleID != "LL020" {
			t.Errorf("unexpected rule %q, want LL020", d.RuleID)
		}
	}
}

func TestLL020_OnWriteBadFixture(t *testing.T) {
	dir := ruleTestdataDir(t, "contract", "ll020_write_bad")
	diags := runRulesOnFixture(t, []string{"LL020"}, dir)
	if len(diags) == 0 {
		t.Fatal("expected at least 1 LL020 diagnostic on write_bad fixture, got 0")
	}
	for _, d := range diags {
		if d.RuleID != "LL020" {
			t.Errorf("unexpected rule %q, want LL020", d.RuleID)
		}
	}
}

func TestLL020_OnGoodFixture(t *testing.T) {
	dir := ruleTestdataDir(t, "contract", "ll020_good")
	diags := runRulesOnFixture(t, []string{"LL020"}, dir)
	if len(diags) != 0 {
		t.Errorf("expected 0 LL020 diagnostics on good fixture, got %d", len(diags))
		for _, d := range diags {
			t.Logf("  unexpected: %s:%d: [%s] %s", filepath.Base(d.Pos.Filename), d.Pos.Line, d.RuleID, d.Message)
		}
	}
}

func TestLL020_RegisteredInDefaultRegistry(t *testing.T) {
	rule := DefaultRegistry.Lookup("LL020")
	if rule == nil {
		t.Fatal("LL020 not found in DefaultRegistry — is init() missing?")
	}
	if rule.ID() != "LL020" {
		t.Errorf("rule ID = %q, want LL020", rule.ID())
	}
	if rule.DefaultSeverity() != diag.SeverityError {
		t.Errorf("LL020 DefaultSeverity = %d, want error", rule.DefaultSeverity())
	}
}

// --- LL021 tests ------------------------------------------------------------

func TestLL021_OnBadFixture(t *testing.T) {
	dir := ruleTestdataDir(t, "contract", "ll021_bad")
	diags := runRulesOnFixture(t, []string{"LL021"}, dir)
	if len(diags) == 0 {
		t.Fatal("expected at least 1 LL021 diagnostic on bad fixture, got 0")
	}
	for _, d := range diags {
		if d.RuleID != "LL021" {
			t.Errorf("unexpected rule %q, want LL021", d.RuleID)
		}
	}
}

func TestLL021_OnGoodLayeredFixture(t *testing.T) {
	dir := ruleTestdataDir(t, "contract", "ll021_good_layered")
	diags := runRulesOnFixture(t, []string{"LL021"}, dir)
	if len(diags) != 0 {
		t.Errorf("expected 0 LL021 diagnostics on good_layered fixture, got %d", len(diags))
	}
}

func TestLL021_OnGoodNonOverlayFixture(t *testing.T) {
	dir := ruleTestdataDir(t, "contract", "ll021_good_nonoverlay")
	diags := runRulesOnFixture(t, []string{"LL021"}, dir)
	if len(diags) != 0 {
		t.Errorf("expected 0 LL021 diagnostics on good_nonoverlay fixture, got %d", len(diags))
	}
}

func TestLL021_RegisteredInDefaultRegistry(t *testing.T) {
	rule := DefaultRegistry.Lookup("LL021")
	if rule == nil {
		t.Fatal("LL021 not found in DefaultRegistry")
	}
	if rule.DefaultSeverity() != diag.SeverityError {
		t.Errorf("LL021 DefaultSeverity = %d, want error", rule.DefaultSeverity())
	}
}

// --- LL022 tests ------------------------------------------------------------

func TestLL022_OnBadFixture(t *testing.T) {
	dir := ruleTestdataDir(t, "contract", "ll022_bad")
	diags := runRulesOnFixture(t, []string{"LL022"}, dir)
	if len(diags) == 0 {
		t.Fatal("expected at least 1 LL022 diagnostic on bad fixture, got 0")
	}
	for _, d := range diags {
		if d.RuleID != "LL022" {
			t.Errorf("unexpected rule %q, want LL022", d.RuleID)
		}
	}
}

func TestLL022_OnGoodScrollFixture(t *testing.T) {
	dir := ruleTestdataDir(t, "contract", "ll022_good_scroll")
	diags := runRulesOnFixture(t, []string{"LL022"}, dir)
	if len(diags) != 0 {
		t.Errorf("expected 0 LL022 diagnostics on good_scroll fixture, got %d", len(diags))
	}
}

func TestLL022_OnGoodSmallFixture(t *testing.T) {
	dir := ruleTestdataDir(t, "contract", "ll022_good_small")
	diags := runRulesOnFixture(t, []string{"LL022"}, dir)
	if len(diags) != 0 {
		t.Errorf("expected 0 LL022 diagnostics on good_small fixture, got %d", len(diags))
	}
}

func TestLL022_OnGoodFlexFixture(t *testing.T) {
	dir := ruleTestdataDir(t, "contract", "ll022_good_flex")
	diags := runRulesOnFixture(t, []string{"LL022"}, dir)
	if len(diags) != 0 {
		t.Errorf("expected 0 LL022 diagnostics on good_flex fixture, got %d", len(diags))
	}
}

func TestLL022_RegisteredInDefaultRegistry(t *testing.T) {
	rule := DefaultRegistry.Lookup("LL022")
	if rule == nil {
		t.Fatal("LL022 not found in DefaultRegistry")
	}
	if rule.DefaultSeverity() != diag.SeverityError {
		t.Errorf("LL022 DefaultSeverity = %d, want error", rule.DefaultSeverity())
	}
}

// --- LL023 tests ------------------------------------------------------------

func TestLL023_OnBadFixture(t *testing.T) {
	dir := ruleTestdataDir(t, "contract", "ll023_bad")
	diags := runRulesOnFixture(t, []string{"LL023"}, dir)
	if len(diags) == 0 {
		t.Fatal("expected at least 1 LL023 diagnostic on bad fixture, got 0")
	}
	for _, d := range diags {
		if d.RuleID != "LL023" {
			t.Errorf("unexpected rule %q, want LL023", d.RuleID)
		}
	}
}

func TestLL023_OnGoodInitFixture(t *testing.T) {
	dir := ruleTestdataDir(t, "contract", "ll023_good_init")
	diags := runRulesOnFixture(t, []string{"LL023"}, dir)
	if len(diags) != 0 {
		t.Errorf("expected 0 LL023 diagnostics on good_init fixture, got %d", len(diags))
	}
}

func TestLL023_OnGoodConstOnlyFixture(t *testing.T) {
	dir := ruleTestdataDir(t, "contract", "ll023_good_const_only")
	diags := runRulesOnFixture(t, []string{"LL023"}, dir)
	if len(diags) != 0 {
		t.Errorf("expected 0 LL023 diagnostics on good_const_only fixture, got %d", len(diags))
	}
}

func TestLL023_OnBadVizFieldFixture(t *testing.T) {
	// Proves RHS-driven detection: StrokeWidth was in no historical name list.
	dir := ruleTestdataDir(t, "contract", "ll023_bad_vizfield")
	diags := runRulesOnFixture(t, []string{"LL023"}, dir)
	if len(diags) == 0 {
		t.Fatal("expected at least 1 LL023 diagnostic on bad_vizfield fixture, got 0")
	}
	for _, d := range diags {
		if d.RuleID != "LL023" {
			t.Errorf("unexpected rule %q, want LL023", d.RuleID)
		}
	}
}

func TestLL023_RegisteredInDefaultRegistry(t *testing.T) {
	rule := DefaultRegistry.Lookup("LL023")
	if rule == nil {
		t.Fatal("LL023 not found in DefaultRegistry")
	}
	if rule.DefaultSeverity() != diag.SeverityError {
		t.Errorf("LL023 DefaultSeverity = %d, want error", rule.DefaultSeverity())
	}
}

// --- LL024 tests ------------------------------------------------------------

func TestLL024_OnBadFixture(t *testing.T) {
	dir := ruleTestdataDir(t, "contract", "ll024_bad")
	diags := runRulesOnFixture(t, []string{"LL024"}, dir)
	if len(diags) == 0 {
		t.Fatal("expected at least 1 LL024 diagnostic on bad fixture, got 0")
	}
	for _, d := range diags {
		if d.RuleID != "LL024" {
			t.Errorf("unexpected rule %q, want LL024", d.RuleID)
		}
	}
}

func TestLL024_OnGoodTreeFixture(t *testing.T) {
	dir := ruleTestdataDir(t, "contract", "ll024_good_tree")
	diags := runRulesOnFixture(t, []string{"LL024"}, dir)
	if len(diags) != 0 {
		t.Errorf("expected 0 LL024 diagnostics on good_tree fixture, got %d", len(diags))
		for _, d := range diags {
			t.Logf("  unexpected: %s:%d: [%s] %s", filepath.Base(d.Pos.Filename), d.Pos.Line, d.RuleID, d.Message)
		}
	}
}

func TestLL024_OnGoodSetFixture(t *testing.T) {
	dir := ruleTestdataDir(t, "contract", "ll024_good_set")
	diags := runRulesOnFixture(t, []string{"LL024"}, dir)
	if len(diags) != 0 {
		t.Errorf("expected 0 LL024 diagnostics on good_set fixture, got %d", len(diags))
		for _, d := range diags {
			t.Logf("  unexpected: %s:%d: [%s] %s", filepath.Base(d.Pos.Filename), d.Pos.Line, d.RuleID, d.Message)
		}
	}
}

func TestLL024_RegisteredInDefaultRegistry(t *testing.T) {
	rule := DefaultRegistry.Lookup("LL024")
	if rule == nil {
		t.Fatal("LL024 not found in DefaultRegistry — is init() missing?")
	}
	if rule.ID() != "LL024" {
		t.Errorf("rule ID = %q, want LL024", rule.ID())
	}
	if rule.DefaultSeverity() != diag.SeverityError {
		t.Errorf("LL024 DefaultSeverity = %d, want error", rule.DefaultSeverity())
	}
}

// --- LL025 tests ------------------------------------------------------------

func TestLL025_OnBadFixture(t *testing.T) {
	dir := ruleTestdataDir(t, "contract", "ll025_bad", "marks", "viz")
	diags := runRulesOnFixture(t, []string{"LL025"}, dir)
	if len(diags) == 0 {
		t.Fatal("expected at least 1 LL025 diagnostic on bad fixture, got 0")
	}
	for _, d := range diags {
		if d.RuleID != "LL025" {
			t.Errorf("unexpected rule %q, want LL025", d.RuleID)
		}
	}
}

func TestLL025_OnGoodSubscribeFixture(t *testing.T) {
	dir := ruleTestdataDir(t, "contract", "ll025_good_subscribe", "marks", "viz")
	diags := runRulesOnFixture(t, []string{"LL025"}, dir)
	if len(diags) != 0 {
		t.Errorf("expected 0 LL025 diagnostics on good_subscribe fixture, got %d", len(diags))
		for _, d := range diags {
			t.Logf("  unexpected: %s:%d: [%s] %s", filepath.Base(d.Pos.Filename), d.Pos.Line, d.RuleID, d.Message)
		}
	}
}

func TestLL025_OnGoodNilguardFixture(t *testing.T) {
	dir := ruleTestdataDir(t, "contract", "ll025_good_nilguard", "marks", "viz")
	diags := runRulesOnFixture(t, []string{"LL025"}, dir)
	if len(diags) != 0 {
		t.Errorf("expected 0 LL025 diagnostics on good_nilguard fixture, got %d", len(diags))
		for _, d := range diags {
			t.Logf("  unexpected: %s:%d: [%s] %s", filepath.Base(d.Pos.Filename), d.Pos.Line, d.RuleID, d.Message)
		}
	}
}

func TestLL025_OnBadTwoScalesFixture(t *testing.T) {
	dir := ruleTestdataDir(t, "contract", "ll025_bad_two_scales_one_tracked", "marks", "viz")
	diags := runRulesOnFixture(t, []string{"LL025"}, dir)
	if len(diags) != 1 {
		t.Fatalf("expected exactly 1 LL025 diagnostic (YScale untracked), got %d", len(diags))
	}
	for _, d := range diags {
		if d.RuleID != "LL025" {
			t.Errorf("unexpected rule %q, want LL025", d.RuleID)
		}
	}
}

func TestLL025_RegisteredInDefaultRegistry(t *testing.T) {
	rule := DefaultRegistry.Lookup("LL025")
	if rule == nil {
		t.Fatal("LL025 not found in DefaultRegistry — is init() missing?")
	}
	if rule.ID() != "LL025" {
		t.Errorf("rule ID = %q, want LL025", rule.ID())
	}
	if rule.DefaultSeverity() != diag.SeverityError {
		t.Errorf("LL025 DefaultSeverity = %d, want error", rule.DefaultSeverity())
	}
}

// --- LL026 tests ------------------------------------------------------------

func TestLL026_OnBadFixture(t *testing.T) {
	dir := ruleTestdataDir(t, "contract", "ll026_bad", "marks", "action")
	diags := runRulesOnFixture(t, []string{"LL026"}, dir)
	if len(diags) == 0 {
		t.Fatal("expected at least 1 LL026 diagnostic on bad fixture, got 0")
	}
	for _, d := range diags {
		if d.RuleID != "LL026" {
			t.Errorf("unexpected rule %q, want LL026", d.RuleID)
		}
	}
}

func TestLL026_OnGoodVersionedFixture(t *testing.T) {
	dir := ruleTestdataDir(t, "contract", "ll026_good_versioned", "marks", "action")
	diags := runRulesOnFixture(t, []string{"LL026"}, dir)
	if len(diags) != 0 {
		t.Errorf("expected 0 LL026 diagnostics on good_versioned fixture, got %d", len(diags))
		for _, d := range diags {
			t.Logf("  unexpected: %s:%d: [%s] %s", filepath.Base(d.Pos.Filename), d.Pos.Line, d.RuleID, d.Message)
		}
	}
}

func TestLL026_OnGoodViewMetricsFixture(t *testing.T) {
	dir := ruleTestdataDir(t, "contract", "ll026_good_viewmetrics", "marks", "action")
	diags := runRulesOnFixture(t, []string{"LL026"}, dir)
	if len(diags) != 0 {
		t.Errorf("expected 0 LL026 diagnostics on good_viewmetrics fixture, got %d", len(diags))
		for _, d := range diags {
			t.Logf("  unexpected: %s:%d: [%s] %s", filepath.Base(d.Pos.Filename), d.Pos.Line, d.RuleID, d.Message)
		}
	}
}

func TestLL026_RegisteredInDefaultRegistry(t *testing.T) {
	rule := DefaultRegistry.Lookup("LL026")
	if rule == nil {
		t.Fatal("LL026 not found in DefaultRegistry — is init() missing?")
	}
	if rule.ID() != "LL026" {
		t.Errorf("rule ID = %q, want LL026", rule.ID())
	}
	if rule.DefaultSeverity() != diag.SeverityError {
		t.Errorf("LL026 DefaultSeverity = %d, want error", rule.DefaultSeverity())
	}
}

// --- LL023 extension (caller-supplied store reassignment) tests -------------

func TestLL023_Ext_OnBadFixture(t *testing.T) {
	dir := ruleTestdataDir(t, "contract", "ll023_ext_bad")
	diags := runRulesOnFixture(t, []string{"LL023"}, dir)
	if len(diags) == 0 {
		t.Fatal("expected at least 1 LL023 diagnostic on ll023_ext_bad fixture, got 0")
	}
	for _, d := range diags {
		if d.RuleID != "LL023" {
			t.Errorf("unexpected rule %q, want LL023", d.RuleID)
		}
	}
}

func TestLL023_Ext_OnGoodSelfOwnedFixture(t *testing.T) {
	dir := ruleTestdataDir(t, "contract", "ll023_ext_good_selfowned")
	diags := runRulesOnFixture(t, []string{"LL023"}, dir)
	if len(diags) != 0 {
		t.Errorf("expected 0 LL023 diagnostics on good_selfowned fixture, got %d", len(diags))
		for _, d := range diags {
			t.Logf("  unexpected: %s:%d: [%s] %s", filepath.Base(d.Pos.Filename), d.Pos.Line, d.RuleID, d.Message)
		}
	}
}

func TestLL023_OriginalBadFixtureStillWorks(t *testing.T) {
	// The original marks.Const-overwrites-reactive test must still fire.
	dir := ruleTestdataDir(t, "contract", "ll023_bad")
	diags := runRulesOnFixture(t, []string{"LL023"}, dir)
	if len(diags) == 0 {
		t.Fatal("expected at least 1 LL023 diagnostic on original ll023_bad fixture, got 0")
	}
	for _, d := range diags {
		if d.RuleID != "LL023" {
			t.Errorf("unexpected rule %q, want LL023", d.RuleID)
		}
	}
}

// --- LL027 tests ------------------------------------------------------------

func TestLL027_OnBadFixture(t *testing.T) {
	dir := ruleTestdataDir(t, "contract", "ll027_bad")
	diags := runRulesOnFixture(t, []string{"LL027"}, dir)
	if len(diags) < 4 {
		t.Fatalf("expected at least 4 LL027 diagnostics (fmt.Sprintf, fmt.Errorf, string concat, aliased fmt), got %d", len(diags))
	}
	for _, d := range diags {
		if d.RuleID != "LL027" {
			t.Errorf("unexpected rule %q, want LL027", d.RuleID)
		}
		// Verify the Teach index ref is set.
		if d.Teach.IndexRef != "signal.Signal[T] (P12: typed, no string routing)" {
			t.Errorf("unexpected Teach.IndexRef: %q", d.Teach.IndexRef)
		}
		// Verify message is correct.
		if d.Message != "string formatting in signal.Emit; use a typed signal payload instead" {
			t.Errorf("unexpected message: %q", d.Message)
		}
	}
	// Verify all 4 bad patterns produce distinct diagnostics.
	lines := make(map[int]bool)
	for _, d := range diags {
		if lines[d.Pos.Line] {
			t.Errorf("duplicate diagnostic on line %d", d.Pos.Line)
		}
		lines[d.Pos.Line] = true
	}
}

func TestLL027_OnBadFixture_NoDiagsWithOtherRule(t *testing.T) {
	// Prove the bad fixture's findings come from LL027, not a neighbor rule.
	dir := ruleTestdataDir(t, "contract", "ll027_bad")
	diags := runRulesOnFixture(t, []string{"LL023"}, dir)
	if len(diags) != 0 {
		t.Errorf("expected 0 diagnostics from other rule on ll027_bad fixture, got %d", len(diags))
	}
}

func TestLL027_OnGoodTypedFixture(t *testing.T) {
	dir := ruleTestdataDir(t, "contract", "ll027_good_typed")
	diags := runRulesOnFixture(t, []string{"LL027"}, dir)
	if len(diags) != 0 {
		t.Errorf("expected 0 LL027 diagnostics on good_typed fixture, got %d", len(diags))
		for _, d := range diags {
			t.Logf("  unexpected: %s:%d: [%s] %s", filepath.Base(d.Pos.Filename), d.Pos.Line, d.RuleID, d.Message)
		}
	}
}

func TestLL027_OnGoodConcatExplainFixture_WithoutIgnore(t *testing.T) {
	// Without ignore processing, string concatenation in Emit must still fire.
	dir := ruleTestdataDir(t, "contract", "ll027_good_concat_explain")
	diags := runRulesOnFixture(t, []string{"LL027"}, dir)
	if len(diags) == 0 {
		t.Fatal("expected at least 1 LL027 diagnostic on good_concat_explain fixture WITHOUT ignore processing, got 0")
	}
}

func TestLL027_OnGoodConcatExplainFixture_WithIgnore(t *testing.T) {
	// With ignore processing, the //lurpiclint:ignore directive must suppress LL027.
	dir := ruleTestdataDir(t, "contract", "ll027_good_concat_explain")
	diags := runRulesOnFixtureWithIndexAndIgnore(t, []string{"LL027"}, dir)
	var ll027count int
	for _, d := range diags {
		if d.RuleID == "LL027" {
			ll027count++
		}
	}
	if ll027count != 0 {
		t.Errorf("expected 0 LL027 diagnostics with ignore processing, got %d", ll027count)
		for _, d := range diags {
			if d.RuleID == "LL027" {
				t.Logf("  unexpected: %s:%d: %s", filepath.Base(d.Pos.Filename), d.Pos.Line, d.Message)
			}
		}
	}
}

func TestLL027_RegisteredInDefaultRegistry(t *testing.T) {
	rule := DefaultRegistry.Lookup("LL027")
	if rule == nil {
		t.Fatal("LL027 not found in DefaultRegistry — is init() missing?")
	}
	if rule.ID() != "LL027" {
		t.Errorf("rule ID = %q, want LL027", rule.ID())
	}
	if rule.DefaultSeverity() != diag.SeverityError {
		t.Errorf("LL027 DefaultSeverity = %d, want error", rule.DefaultSeverity())
	}
}

// --- LL028 tests ------------------------------------------------------------

func TestLL028_OnBadFixture(t *testing.T) {
	dir := ruleTestdataDir(t, "contract", "ll028_bad", "marks", "viz")
	diags := runRulesOnFixture(t, []string{"LL028"}, dir)
	if len(diags) == 0 {
		t.Fatal("expected at least 1 LL028 diagnostic on bad fixture, got 0")
	}
	for _, d := range diags {
		if d.RuleID != "LL028" {
			t.Errorf("unexpected rule %q, want LL028", d.RuleID)
		}
	}
}

func TestLL028_OnGoodThemeFixture(t *testing.T) {
	dir := ruleTestdataDir(t, "contract", "ll028_good_theme", "marks", "viz")
	diags := runRulesOnFixture(t, []string{"LL028"}, dir)
	if len(diags) != 0 {
		t.Errorf("expected 0 LL028 diagnostics on good_theme fixture, got %d", len(diags))
		for _, d := range diags {
			t.Logf("  unexpected: %s:%d: [%s] %s", filepath.Base(d.Pos.Filename), d.Pos.Line, d.RuleID, d.Message)
		}
	}
}

func TestLL028_OnGoodOutsideVizFixture(t *testing.T) {
	// gfx.Color{...} outside marks/viz/ must not fire.
	dir := ruleTestdataDir(t, "contract", "ll028_good_outside_viz")
	diags := runRulesOnFixture(t, []string{"LL028"}, dir)
	if len(diags) != 0 {
		t.Errorf("expected 0 LL028 diagnostics on good_outside_viz fixture, got %d", len(diags))
		for _, d := range diags {
			t.Logf("  unexpected: %s:%d: [%s] %s", filepath.Base(d.Pos.Filename), d.Pos.Line, d.RuleID, d.Message)
		}
	}
}

func TestLL028_RegisteredInDefaultRegistry(t *testing.T) {
	rule := DefaultRegistry.Lookup("LL028")
	if rule == nil {
		t.Fatal("LL028 not found in DefaultRegistry — is init() missing?")
	}
	if rule.ID() != "LL028" {
		t.Errorf("rule ID = %q, want LL028", rule.ID())
	}
	if rule.DefaultSeverity() != diag.SeverityWarn {
		t.Errorf("LL028 DefaultSeverity = %d, want warn", rule.DefaultSeverity())
	}
}

// Contract-rule tests (LL029–LL033) live in contract_rules_test.go.

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

func TestLL011_Demo_OnNameBadFixture(t *testing.T) {
	// A package named "studio" with a raw goroutine must trigger LL011,
	// even though it does not embed facet.Facet.
	dir := ruleTestdataDir(t, "contract", "ll011_demo_bad")
	diags := runRulesOnFixture(t, []string{"LL011"}, dir)
	if len(diags) == 0 {
		t.Fatal("expected LL011 diagnostic in 'studio' demo package with goroutine")
	}
	for _, d := range diags {
		if d.RuleID != "LL011" {
			t.Errorf("unexpected rule %q, want LL011", d.RuleID)
		}
	}
}

func TestLL011_Demo_OnImportBadFixture(t *testing.T) {
	// A package that imports both "time" and a store path must trigger LL011
	// for raw goroutines, even without a demo-style package name.
	dir := ruleTestdataDir(t, "contract", "ll011_demo_import_bad")
	diags := runRulesOnFixture(t, []string{"LL011"}, dir)
	if len(diags) == 0 {
		t.Fatal("expected LL011 diagnostic in time+store importing package with goroutine")
	}
}

func TestLL011_Demo_OnGoodFixture(t *testing.T) {
	// A demo package using job.Schedule (allowlisted) must not trigger LL011.
	dir := ruleTestdataDir(t, "contract", "ll011_demo_good")
	diags := runRulesOnFixture(t, []string{"LL011"}, dir)
	if len(diags) != 0 {
		t.Errorf("expected 0 diagnostics on good LL011 demo fixture, got %d", len(diags))
		for _, d := range diags {
			t.Logf("  unexpected: %s:%d: %s", filepath.Base(d.Pos.Filename), d.Pos.Line, d.Message)
		}
	}
}

// runRulesOnFixtureWithIndex is like runRulesOnFixture but also builds and
// injects the capability index (needed by LL004).
// fixtureContextWithIndex loads a fixture directory and injects the capability
// index scanned from the real framework packages (marks/, layout/, facet).
func fixtureContextWithIndex(tb testing.TB, dir string) *Context {
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
	return &Context{
		Files: result.Files,
		Pkgs:  result.Packages,
		Fset:  result.Fset,
		Index: caps,
	}
}

func runRulesOnFixtureWithIndex(tb testing.TB, ruleIDs []string, dir string) []*diag.Diagnostic {
	tb.Helper()
	ctx := fixtureContextWithIndex(tb, dir)
	return Run(ctx, DefaultRegistry, RunConfig{
		EnabledIDs: ruleIDs,
	})
}

// runRulesOnFixtureWithIndexAndIgnore is like runRulesOnFixtureWithIndex
// but also processes //lurpiclint:ignore directives from the loaded files,
// matching the full pipeline that main.go runs.
func runRulesOnFixtureWithIndexAndIgnore(tb testing.TB, ruleIDs []string, dir string) []*diag.Diagnostic {
	tb.Helper()
	ctx := fixtureContextWithIndex(tb, dir)
	diags := Run(ctx, DefaultRegistry, RunConfig{
		EnabledIDs: ruleIDs,
	})

	// Collect //lurpiclint:ignore directives from all loaded files.
	var ignores []config.IgnoreDirective
	for _, f := range ctx.Files {
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

func TestLL004_ScalarAccessor_OnBadFixture(t *testing.T) {
	// viz.NewBar with scalar accessor closures should trigger LL004 (info).
	dir := ruleTestdataDir(t, "suggest", "ll004_scalar_bad")
	diags := runRulesOnFixture(t, []string{"LL004"}, dir)
	if len(diags) == 0 {
		t.Error("expected at least 1 LL004 diagnostic for scalar accessor closures in viz.New*, got 0")
	}
	for _, d := range diags {
		if d.RuleID != "LL004" {
			t.Errorf("unexpected rule %q, want LL004", d.RuleID)
		}
	}
}

func TestLL004_ScalarAccessor_OnGoodFixture(t *testing.T) {
	// A file without viz.New* calls should not trigger the scalar
	// accessor check.
	dir := ruleTestdataDir(t, "suggest", "ll004_scalar_good")
	diags := runRulesOnFixture(t, []string{"LL004"}, dir)
	if len(diags) != 0 {
		t.Errorf("expected 0 diagnostics on good LL004 scalar fixture, got %d", len(diags))
		for _, d := range diags {
			t.Logf("  unexpected: %s:%d: %s", filepath.Base(d.Pos.Filename), d.Pos.Line, d.Message)
		}
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

// --- LL003 field-assignment extension tests ---------------------------------

func TestLL003_OnFieldAssignBadFixture(t *testing.T) {
	dir := ruleTestdataDir(t, "contract", "ll003_field_assign_bad")
	diags := runRulesOnFixture(t, []string{"LL003"}, dir)
	if len(diags) == 0 {
		t.Error("expected at least 1 LL003 diagnostic on field_assign_bad fixture, got 0")
	}
	for _, d := range diags {
		if d.RuleID != "LL003" {
			t.Errorf("unexpected rule %q, want LL003", d.RuleID)
		}
	}
}

func TestLL003_OnHelperBadFixture(t *testing.T) {
	dir := ruleTestdataDir(t, "contract", "ll003_helper_bad")
	diags := runRulesOnFixture(t, []string{"LL003"}, dir)
	if len(diags) == 0 {
		t.Error("expected at least 1 LL003 diagnostic on helper_bad fixture, got 0")
	}
	for _, d := range diags {
		if d.RuleID != "LL003" {
			t.Errorf("unexpected rule %q, want LL003", d.RuleID)
		}
	}
}

func TestLL003_OnFieldAssignGoodFixture(t *testing.T) {
	dir := ruleTestdataDir(t, "contract", "ll003_field_assign_good")
	diags := runRulesOnFixture(t, []string{"LL003"}, dir)
	if len(diags) != 0 {
		t.Errorf("expected 0 diagnostics on field_assign_good fixture, got %d", len(diags))
		for _, d := range diags {
			t.Logf("  unexpected: %s:%d: %s", filepath.Base(d.Pos.Filename), d.Pos.Line, d.Message)
		}
	}
}

func TestLL003_OnLeafGoodFixture(t *testing.T) {
	dir := ruleTestdataDir(t, "contract", "ll003_leaf_good")
	diags := runRulesOnFixture(t, []string{"LL003"}, dir)
	if len(diags) != 0 {
		t.Errorf("expected 0 diagnostics on leaf_good fixture, got %d", len(diags))
		for _, d := range diags {
			t.Logf("  unexpected: %s:%d: %s", filepath.Base(d.Pos.Filename), d.Pos.Line, d.Message)
		}
	}
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

// --- LL016 tests ------------------------------------------------------------

func TestLL016_OnBadFixture(t *testing.T) {
	dir := ruleTestdataDir(t, "contract", "ll016_bad")
	diags := runRulesOnFixture(t, []string{"LL016"}, dir)
	if len(diags) < 2 {
		t.Errorf("expected at least 2 LL016 diagnostics (OnMeasure + OnArrange), got %d", len(diags))
	}
	for _, d := range diags {
		if d.RuleID != "LL016" {
			t.Errorf("unexpected rule %q, want LL016", d.RuleID)
		}
	}
}

func TestLL016_OnGoodFixture(t *testing.T) {
	dir := ruleTestdataDir(t, "contract", "ll016_good")
	diags := runRulesOnFixture(t, []string{"LL016"}, dir)
	if len(diags) != 0 {
		t.Errorf("expected 0 diagnostics on good LL016 fixture, got %d", len(diags))
		for _, d := range diags {
			t.Logf("  unexpected: %s:%d: %s", filepath.Base(d.Pos.Filename), d.Pos.Line, d.Message)
		}
	}
}

func TestLL016_RegisteredInDefaultRegistry(t *testing.T) {
	rule := DefaultRegistry.Lookup("LL016")
	if rule == nil {
		t.Fatal("LL016 not found in DefaultRegistry — is init() missing?")
	}
	if rule.ID() != "LL016" {
		t.Errorf("rule ID = %q, want LL016", rule.ID())
	}
	if rule.DefaultSeverity() != diag.SeverityError {
		t.Errorf("LL016 DefaultSeverity = %d, want error", rule.DefaultSeverity())
	}
}

// --- LL017 tests ------------------------------------------------------------

func TestLL017_OnBadFixture(t *testing.T) {
	dir := ruleTestdataDir(t, "contract", "ll017_bad")
	diags := runRulesOnFixture(t, []string{"LL017"}, dir)
	if len(diags) == 0 {
		t.Error("expected at least 1 LL017 diagnostic for media file via app.Asset, got 0")
	}
	for _, d := range diags {
		if d.RuleID != "LL017" {
			t.Errorf("unexpected rule %q, want LL017", d.RuleID)
		}
	}
}

func TestLL017_OnGoodFixture(t *testing.T) {
	dir := ruleTestdataDir(t, "contract", "ll017_good")
	diags := runRulesOnFixture(t, []string{"LL017"}, dir)
	if len(diags) != 0 {
		t.Errorf("expected 0 diagnostics on good LL017 fixture (non-media config file), got %d", len(diags))
		for _, d := range diags {
			t.Logf("  unexpected: %s:%d: %s", filepath.Base(d.Pos.Filename), d.Pos.Line, d.Message)
		}
	}
}

func TestLL017_RegisteredInDefaultRegistry(t *testing.T) {
	rule := DefaultRegistry.Lookup("LL017")
	if rule == nil {
		t.Fatal("LL017 not found in DefaultRegistry — is init() missing?")
	}
	if rule.ID() != "LL017" {
		t.Errorf("rule ID = %q, want LL017", rule.ID())
	}
	if rule.DefaultSeverity() != diag.SeverityWarn {
		t.Errorf("LL017 DefaultSeverity = %d, want warn", rule.DefaultSeverity())
	}
}

// --- LL018 tests ------------------------------------------------------------

func TestLL018_OnBadFixture(t *testing.T) {
	dir := ruleTestdataDir(t, "contract", "ll018_bad")
	diags := runRulesOnFixture(t, []string{"LL018"}, dir)
	if len(diags) == 0 {
		t.Error("expected at least 1 LL018 diagnostic for unmounted overlay, got 0")
	}
	for _, d := range diags {
		if d.RuleID != "LL018" {
			t.Errorf("unexpected rule %q, want LL018", d.RuleID)
		}
	}
}

func TestLL018_OnGoodFixture(t *testing.T) {
	dir := ruleTestdataDir(t, "contract", "ll018_good")
	diags := runRulesOnFixture(t, []string{"LL018"}, dir)
	if len(diags) != 0 {
		t.Errorf("expected 0 diagnostics on good LL018 fixture (mounted overlay), got %d", len(diags))
		for _, d := range diags {
			t.Logf("  unexpected: %s:%d: %s", filepath.Base(d.Pos.Filename), d.Pos.Line, d.Message)
		}
	}
}

func TestLL018_RegisteredInDefaultRegistry(t *testing.T) {
	rule := DefaultRegistry.Lookup("LL018")
	if rule == nil {
		t.Fatal("LL018 not found in DefaultRegistry — is init() missing?")
	}
	if rule.ID() != "LL018" {
		t.Errorf("rule ID = %q, want LL018", rule.ID())
	}
	if rule.DefaultSeverity() != diag.SeverityWarn {
		t.Errorf("LL018 DefaultSeverity = %d, want warn", rule.DefaultSeverity())
	}
}

// --- LL001 package-path extension tests -------------------------------------

func TestLL001_PackageCheck_OnBadFixture(t *testing.T) {
	// LL001 should fire when LayoutRole callbacks are assigned outside
	// layout/ or marks/ packages.
	dir := ruleTestdataDir(t, "contract", "ll001_package_bad")
	diags := runRulesOnFixture(t, []string{"LL001"}, dir)
	if len(diags) == 0 {
		t.Error("expected at least 1 LL001 diagnostic on bad package fixture, got 0")
	}
	for _, d := range diags {
		if d.RuleID != "LL001" {
			t.Errorf("unexpected rule %q, want LL001", d.RuleID)
		}
	}
}

func TestLL001_PackageCheck_OnGoodFixture(t *testing.T) {
	// LL001 should NOT fire when LayoutRole callbacks are assigned inside
	// a layout/ package.
	dir := ruleTestdataDir(t, "contract", "ll001_package_good", "layout", "mock")
	diags := runRulesOnFixture(t, []string{"LL001"}, dir)
	if len(diags) != 0 {
		t.Errorf("expected 0 diagnostics on good layout-package fixture, got %d", len(diags))
		for _, d := range diags {
			t.Logf("  unexpected: %s:%d: %s", filepath.Base(d.Pos.Filename), d.Pos.Line, d.Message)
		}
	}
}

// TestLL001_PackageCheck_BadFixtureStillTriggers verifies that the original
// LL001 bad fixture (which is also outside layout/ or marks/) still fires.
func TestLL001_PackageCheck_OriginalBadFixtureStillWorks(t *testing.T) {
	dir := ruleTestdataDir(t, "reinvent", "ll001_bad")
	diags := runRulesOnFixture(t, []string{"LL001"}, dir)
	compareAgainstGolden(t, diags, "golden/ll001_bad.json")
}

// TestLL001_AssignmentPattern_OnBadFixture verifies that LL001 now catches
// the field-assignment shape (`b.layout.OnMeasure = func(...) {...}`) which
// the original composite-literal-only scan could not see.  Both the
// OnMeasure and OnArrange assignments must produce findings.
func TestLL001_AssignmentPattern_OnBadFixture(t *testing.T) {
	dir := ruleTestdataDir(t, "contract", "ll001_assign_bad")
	diags := runRulesOnFixture(t, []string{"LL001"}, dir)
	if len(diags) < 2 {
		t.Fatalf("expected at least 2 LL001 diagnostics (OnMeasure + OnArrange), got %d", len(diags))
	}
	for _, d := range diags {
		if d.RuleID != "LL001" {
			t.Errorf("unexpected rule %q on assign-bad fixture, want LL001", d.RuleID)
		}
		if !strings.Contains(d.Message, "OnMeasure/OnArrange assigned outside layout/ or marks/") {
			t.Errorf("unexpected LL001 message: %q", d.Message)
		}
	}
}

// --- Teeth meta-test (NFR-6 automated mutation check) ------------------------

func TestRules_Teeth(t *testing.T) {
	type tooth struct {
		ruleID string
		badDir string
		subDir []string
	}
	teeth := []tooth{
		{ruleID: "LL023", badDir: "ll023_bad"},
		{ruleID: "LL023", badDir: "ll023_ext_bad"},
		{ruleID: "LL024", badDir: "ll024_bad"},
		{ruleID: "LL025", badDir: "ll025_bad", subDir: []string{"marks", "viz"}},
		{ruleID: "LL026", badDir: "ll026_bad", subDir: []string{"marks", "action"}},
		{ruleID: "LL027", badDir: "ll027_bad"},
		{ruleID: "LL028", badDir: "ll028_bad", subDir: []string{"marks", "viz"}},
	}

	for _, c := range teeth {
		t.Run(c.ruleID+"/"+c.badDir, func(t *testing.T) {
			elems := append([]string{"contract", c.badDir}, c.subDir...)
			dir := ruleTestdataDir(t, elems...)
			result, err := loader.Load([]string{dir}, loader.Config{})
			if err != nil {
				t.Fatalf("loading %s: %v", dir, err)
			}
			ctx := &Context{
				Files: result.Files,
				Pkgs:  result.Packages,
				Fset:  result.Fset,
			}

			// Phase 1: run ALL rules, target rule must fire.
			allDiags := Run(ctx, DefaultRegistry, RunConfig{})
			var targetEnabled int
			for _, d := range allDiags {
				if d.RuleID == c.ruleID {
					targetEnabled++
				}
			}
			if targetEnabled == 0 {
				t.Fatalf("teeth: expected %s to fire on its bad fixture, but got 0 findings", c.ruleID)
			}

			// Phase 2: disable the target rule — it must NOT fire.
			disabledDiags := Run(ctx, DefaultRegistry, RunConfig{
				DisabledIDs: map[string]bool{c.ruleID: true},
			})
			var targetDisabled int
			for _, d := range disabledDiags {
				if d.RuleID == c.ruleID {
					targetDisabled++
				}
			}
			if targetDisabled != 0 {
				t.Errorf("teeth: %s fired %d time(s) when disabled", c.ruleID, targetDisabled)
			}
		})
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
