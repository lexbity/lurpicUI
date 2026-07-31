package rules

import (
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/capindex"
	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/config"
	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/diag"
	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/loader"
)

// contractRule describes one capability-contract rule (LL029–LL033) and its
// fixture layout.  All five rules share the same teeth shape (unwired fires,
// wired silent, nolint-with-reason silent, placeholder-nolint warns), so the
// common cases are table-driven here.  Rule-specific guards live in their own
// tests below.
type contractRule struct {
	ruleID   string
	dirBase  string // testdata/ll0XX/
	category string // marks/<category>/ fixture suffix
}

var contractRules = []contractRule{
	{ruleID: "LL029", dirBase: "ll029", category: "databound"},
	{ruleID: "LL030", dirBase: "ll030", category: "anchors"},
	{ruleID: "LL031", dirBase: "ll031", category: "groups"},
	{ruleID: "LL032", dirBase: "ll032", category: "access"},
	{ruleID: "LL033", dirBase: "ll033", category: "focus"},
}

// fixtureDir returns the absolute path to the rule's <variant>/marks/<category>
// fixture package.
func (c contractRule) fixtureDir(t *testing.T, variant string) string {
	t.Helper()
	return ruleTestdataDir(t, c.dirBase, variant, "marks", c.category)
}

func TestContractRules_Teeth_UnwiredFires(t *testing.T) {
	for _, c := range contractRules {
		t.Run(c.ruleID, func(t *testing.T) {
			// A mark declaring the capability with no contracttest helper
			// application must produce an error-severity diagnostic.
			dir := c.fixtureDir(t, "unwired")
			diags := runRulesOnFixture(t, []string{c.ruleID}, dir)
			if len(diags) == 0 {
				t.Fatalf("expected at least 1 %s diagnostic for unwired mark", c.ruleID)
			}
			for _, d := range diags {
				if d.RuleID != c.ruleID {
					t.Errorf("unexpected rule %q, want %s", d.RuleID, c.ruleID)
				}
				if d.Severity != diag.SeverityError {
					t.Errorf("%s severity = %s, want error", c.ruleID, d.Severity)
				}
			}
		})
	}
}

func TestContractRules_Teeth_WiredSilent(t *testing.T) {
	for _, c := range contractRules {
		t.Run(c.ruleID, func(t *testing.T) {
			// The same mark with the matching contracttest helper applied must
			// be silent.
			dir := c.fixtureDir(t, "wired")
			diags := runRulesOnFixture(t, []string{c.ruleID}, dir)
			if len(diags) != 0 {
				t.Errorf("expected 0 diagnostics on wired %s fixture, got %d", c.ruleID, len(diags))
				for _, d := range diags {
					t.Logf("  unexpected: %s:%d: %s", filepath.Base(d.Pos.Filename), d.Pos.Line, d.Message)
				}
			}
		})
	}
}

func TestContractRules_Teeth_NolintWithReason(t *testing.T) {
	for _, c := range contractRules {
		t.Run(c.ruleID, func(t *testing.T) {
			// A //nolint:<ruleID> directive with a substantive reason
			// suppresses the missing-helper diagnostic.
			dir := c.fixtureDir(t, "nolint_ok")
			diags := runRulesOnFixture(t, []string{c.ruleID}, dir)
			if len(diags) != 0 {
				t.Errorf("expected 0 diagnostics on nolint-with-reason %s fixture, got %d", c.ruleID, len(diags))
				for _, d := range diags {
					t.Logf("  unexpected: %s:%d: %s", filepath.Base(d.Pos.Filename), d.Pos.Line, d.Message)
				}
			}
		})
	}
}

func TestContractRules_Teeth_NolintEmptyWarns(t *testing.T) {
	for _, c := range contractRules {
		t.Run(c.ruleID, func(t *testing.T) {
			// A //nolint:<ruleID> directive with a placeholder reason ("todo")
			// must produce a separate warning diagnostic.
			dir := c.fixtureDir(t, "nolint_bad")
			diags := runRulesOnFixture(t, []string{c.ruleID}, dir)
			if len(diags) != 1 {
				t.Fatalf("expected 1 warning diagnostic for placeholder nolint, got %d", len(diags))
			}
			d := diags[0]
			if d.RuleID != c.ruleID {
				t.Errorf("diagnostic RuleID = %q, want %s", d.RuleID, c.ruleID)
			}
			if d.Severity != diag.SeverityWarn {
				t.Errorf("diagnostic Severity = %s, want warn", d.Severity)
			}
			if !strings.Contains(d.Message, "nolint") {
				t.Errorf("diagnostic Message = %q, want it to mention nolint", d.Message)
			}
		})
	}
}

// TestContractRules_BaselineMatchesBacklog verifies that lurpiclint-baseline.json
// records exactly the current LL029–LL033 backlog: every contract entry is
// non-stale (the rule still fires) and every contract finding is baselined
// (nothing unrecorded).  This is the teeth test for the baseline ratchet that
// --fail-on-stale-baseline enforces in CI.
func TestContractRules_BaselineMatchesBacklog(t *testing.T) {
	root := testRepoRoot(t)
	ctx := realFrameworkContext(t, root)

	diags := Run(ctx, DefaultRegistry, RunConfig{
		EnabledIDs: []string{"LL029", "LL030", "LL031", "LL032", "LL033"},
	})

	baseline, err := config.LoadBaseline(filepath.Join(root, "lurpiclint-baseline.json"))
	if err != nil {
		t.Fatal(err)
	}

	filtered, stale := config.SuppressByBaseline(diags, baseline)

	var staleContract []config.BaselineEntry
	for _, e := range stale {
		if isContractRuleID(e.RuleID) {
			staleContract = append(staleContract, e)
		}
	}
	if len(staleContract) != 0 {
		t.Errorf("baseline has %d stale LL029–LL033 entries (rule no longer fires):", len(staleContract))
		for _, e := range staleContract {
			t.Logf("  stale: %s %s:%d", e.RuleID, filepath.Base(e.File), e.Line)
		}
	}
	if len(filtered) != 0 {
		t.Errorf("expected 0 un-baselined LL029–LL033 findings, got %d:", len(filtered))
		for _, d := range filtered {
			t.Logf("  un-baselined: %s:%d %s: %s", filepath.Base(d.Pos.Filename), d.Pos.Line, d.RuleID, d.Message)
		}
	}

	// The baseline count for each rule must equal the current finding count.
	for _, c := range contractRules {
		var findings, base int
		for _, d := range diags {
			if d.RuleID == c.ruleID {
				findings++
			}
		}
		for _, e := range baseline.Entries {
			if e.RuleID == c.ruleID {
				base++
			}
		}
		if findings != base {
			t.Errorf("rule %s: findings=%d baseline_entries=%d; baseline must match reality", c.ruleID, findings, base)
		}
	}
}

func isContractRuleID(id string) bool {
	for _, c := range contractRules {
		if c.ruleID == id {
			return true
		}
	}
	return false
}

// realFrameworkContext loads the real framework packages (marks/, layout/),
// merges their _test.go files into the result (mirroring main.go's
// mergeFrameworkTestFiles so contract rules see helper invocations), and
// injects the capability index.
func realFrameworkContext(t *testing.T, root string) *Context {
	t.Helper()
	result, err := loader.Load([]string{
		root + "/marks/...",
		root + "/layout/...",
	}, loader.Config{})
	if err != nil {
		t.Fatal(err)
	}

	testResult, err := loader.Load([]string{
		root + "/marks/...",
		root + "/layout/...",
	}, loader.Config{IncludeTests: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, pkg := range testResult.Packages {
		mainPkg := result.Packages[pkg.Path]
		if mainPkg == nil {
			continue
		}
		for _, pf := range pkg.Files {
			if !strings.HasSuffix(pf.Path, "_test.go") {
				continue
			}
			mainPkg.Files = append(mainPkg.Files, pf)
			result.Files = append(result.Files, pf)
		}
	}
	sortFiles(result.Files, result.Packages)

	capResult, err := loader.Load([]string{
		root + "/marks/...",
		root + "/layout/...",
		root + "/facet",
	}, loader.Config{})
	if err != nil {
		t.Fatal(err)
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

// sortFiles re-sorts result.Files and each package's Files by path, matching
// the deterministic order the loader produces.
func sortFiles(files []*loader.ParsedFile, pkgs map[string]*loader.Package) {
	for _, pkg := range pkgs {
		sort.Slice(pkg.Files, func(i, j int) bool {
			return pkg.Files[i].Path < pkg.Files[j].Path
		})
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].Path < files[j].Path
	})
}

// --- LL031-specific guards -------------------------------------------------

func TestLL031_FixtureExercisesEmbeddedRoles(t *testing.T) {
	// The ll031 unwired fixture's GroupMark embeds a simulated marks.Core
	// (groupCore) instead of declaring its layout role directly.  The
	// fingerprint MUST traverse the embedded type to see the promoted role;
	// if it does not, GroupMark is not a container and the teeth test would
	// silently skip it rather than exercising the production embed path.
	dir := ruleTestdataDir(t, "ll031", "unwired", "marks", "groups")
	result, err := loader.Load([]string{dir}, loader.Config{})
	if err != nil {
		t.Fatal(err)
	}
	root := testRepoRoot(t)
	caps := capindex.Scan(result, capindex.ScanConfig{
		ModulePath: "codeburg.org/lexbit/lurpicui",
		ModuleRoot: root,
	})
	var groupMark *capindex.Capability
	for i := range caps {
		if caps[i].TypeName == "GroupMark" {
			groupMark = &caps[i]
			break
		}
	}
	if groupMark == nil {
		t.Fatal("GroupMark not found in ll031 unwired fixture capindex")
	}
	if !groupMark.Fingerprint.EmbedsFacet {
		t.Error("GroupMark must embed facet.Facet (transitively via the simulated marks.Core)")
	}
	hasLayout := false
	for _, r := range groupMark.Fingerprint.Roles {
		if r == "layout" {
			hasLayout = true
			break
		}
	}
	if !hasLayout {
		t.Errorf("GroupMark roles = %v, want a promoted layout role from the embedded groupCore", groupMark.Fingerprint.Roles)
	}
	if !groupMark.Fingerprint.IsContainer {
		t.Errorf("GroupMark MUST be IsContainer=true (embedded role-field path); roles=%v", groupMark.Fingerprint.Roles)
	}
}

func TestLL031_Teeth_LeafWithIsContainerDoesNotFire(t *testing.T) {
	// A mark whose fingerprint classifies it as a container (embeds
	// facet.Facet + layout role) but which declares no Children() method is
	// a leaf — the rule MUST NOT fire (R-3 over-fire guard).
	dir := ruleTestdataDir(t, "ll031", "leaf", "marks", "groups")
	diags := runRulesOnFixture(t, []string{"LL031"}, dir)
	if len(diags) != 0 {
		t.Errorf("expected 0 diagnostics on container-fingerprint leaf fixture, got %d", len(diags))
		for _, d := range diags {
			t.Logf("  unexpected: %s:%d: %s", filepath.Base(d.Pos.Filename), d.Pos.Line, d.Message)
		}
	}
}

func TestLL031_Teeth_NonContainerSilent(t *testing.T) {
	// A mark declaring Children() []facet.GroupChild but NOT classified as a
	// container must not fire — the IsContainer gate requires BOTH the
	// fingerprint and the method.  This documents the rule's conservative
	// behaviour (R-3: prefer a false-negative over a false-positive).
	dir := ruleTestdataDir(t, "ll031", "noncontainer", "marks", "groups")
	diags := runRulesOnFixture(t, []string{"LL031"}, dir)
	if len(diags) != 0 {
		t.Errorf("expected 0 diagnostics on non-container Children() mark, got %d", len(diags))
		for _, d := range diags {
			t.Logf("  unexpected: %s:%d: %s", filepath.Base(d.Pos.Filename), d.Pos.Line, d.Message)
		}
	}
}

// --- LL032-specific guard --------------------------------------------------

func TestLL032_Teeth_PartialImplementationDoesNotFire(t *testing.T) {
	// A mark declaring only AccessibilityRole() — no AccessibleName() — is
	// NOT an Accessible implementor and must not fire (false-positive guard).
	dir := ruleTestdataDir(t, "ll032", "partial", "marks", "access")
	diags := runRulesOnFixture(t, []string{"LL032"}, dir)
	if len(diags) != 0 {
		t.Errorf("expected 0 diagnostics on partial Accessible implementation, got %d", len(diags))
		for _, d := range diags {
			t.Logf("  unexpected: %s:%d: %s", filepath.Base(d.Pos.Filename), d.Pos.Line, d.Message)
		}
	}
}

// --- LL033-specific guard --------------------------------------------------

func TestLL033_NoFalsePositiveOnUnrelatedFocusableMethod(t *testing.T) {
	// A Focusable() promoted from an embedded helper struct is NOT the
	// marks.Focusable capability: the rule scans directly-declared methods
	// on the mark type, so it must not fire.
	dir := ruleTestdataDir(t, "ll033", "embedded_helper", "marks", "focus")
	diags := runRulesOnFixture(t, []string{"LL033"}, dir)
	if len(diags) != 0 {
		t.Errorf("expected 0 diagnostics on embedded-helper Focusable fixture, got %d", len(diags))
		for _, d := range diags {
			t.Logf("  unexpected: %s:%d: %s", filepath.Base(d.Pos.Filename), d.Pos.Line, d.Message)
		}
	}
}
