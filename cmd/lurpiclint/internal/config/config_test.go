package config

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"testing"

	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/diag"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Lurpiclint.FailOn != "error" {
		t.Errorf("FailOn = %q, want %q", cfg.Lurpiclint.FailOn, "error")
	}
}

func TestConfig_RuleSeverity(t *testing.T) {
	cfg := Config{
		Rules: map[string]RuleConfig{
			"LL003": {Severity: "error"},
			"LL012": {Severity: "off"},
		},
	}
	if cfg.RuleSeverity("LL003") != "error" {
		t.Errorf("LL003 severity = %q, want %q", cfg.RuleSeverity("LL003"), "error")
	}
	if cfg.RuleSeverity("LL012") != "off" {
		t.Errorf("LL012 severity = %q, want %q", cfg.RuleSeverity("LL012"), "off")
	}
	if cfg.RuleSeverity("LL999") != "" {
		t.Errorf("LL999 severity = %q, want empty", cfg.RuleSeverity("LL999"))
	}
}

func TestConfig_PathExcluded(t *testing.T) {
	cfg := Config{
		Paths: PathsConfig{
			Exclude: []string{"vendor/**", "**/*_gen.go"},
		},
	}
	tests := []struct {
		path     string
		excluded bool
	}{
		{"foo/bar.go", false},
		{"vendor/pkg/file.go", true},
		{"src/code_gen.go", true},
		{"src/foo/bar_gen.go", true},
		{"src/normal.go", false},
	}
	for _, tt := range tests {
		got := cfg.PathExcluded(tt.path)
		if got != tt.excluded {
			t.Errorf("PathExcluded(%q) = %v, want %v", tt.path, got, tt.excluded)
		}
	}
}

func TestParseIgnoreDirectives_Valid(t *testing.T) {
	src := `package p
//lurpiclint:ignore LL003 -- false positive in test
func f() {}`

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	dirs := ParseIgnoreDirectives(fset, f)
	if len(dirs) != 1 {
		t.Fatalf("got %d directives, want 1", len(dirs))
	}
	if dirs[0].RuleID != "LL003" {
		t.Errorf("RuleID = %q, want %q", dirs[0].RuleID, "LL003")
	}
	if dirs[0].Reason != "false positive in test" {
		t.Errorf("Reason = %q, want %q", dirs[0].Reason, "false positive in test")
	}
	if !dirs[0].Valid {
		t.Error("directive should be valid")
	}
}

func TestParseIgnoreDirectives_AllRules(t *testing.T) {
	src := `package p
//lurpiclint:ignore * -- known issue, will fix later
func f() {}`

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	dirs := ParseIgnoreDirectives(fset, f)
	if len(dirs) != 1 {
		t.Fatalf("got %d directives, want 1", len(dirs))
	}
	if dirs[0].RuleID != "*" {
		t.Errorf("RuleID = %q, want %q", dirs[0].RuleID, "*")
	}
	if !dirs[0].Valid {
		t.Error("directive should be valid")
	}
}

func TestParseIgnoreDirectives_MissingReason(t *testing.T) {
	src := `package p
//lurpiclint:ignore LL003
func f() {}`

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	dirs := ParseIgnoreDirectives(fset, f)
	if len(dirs) != 1 {
		t.Fatalf("got %d directives, want 1", len(dirs))
	}
	if dirs[0].Valid {
		t.Error("directive without reason should be invalid")
	}
	if dirs[0].Reason != "" {
		t.Errorf("Reason = %q, want empty", dirs[0].Reason)
	}
}

func TestParseIgnoreDirectives_EmptyBody(t *testing.T) {
	src := `package p
//lurpiclint:ignore
func f() {}`

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	dirs := ParseIgnoreDirectives(fset, f)
	if len(dirs) != 1 {
		t.Fatalf("got %d directives, want 1", len(dirs))
	}
	if dirs[0].Valid {
		t.Error("empty directive should be invalid")
	}
}

func TestSuppressByIgnore_Suppresses(t *testing.T) {
	diags := []*diag.Diagnostic{
		{RuleID: "LL003", Pos: token.Position{Filename: "a.go", Line: 10}},
		{RuleID: "LL001", Pos: token.Position{Filename: "a.go", Line: 20}},
	}
	ignores := []IgnoreDirective{
		{RuleID: "LL003", Reason: "ok", Pos: token.Position{Filename: "a.go", Line: 10}, Valid: true},
	}
	result := SuppressByIgnore(diags, ignores)
	if len(result) != 1 {
		t.Fatalf("got %d diagnostics after suppress, want 1", len(result))
	}
	if result[0].RuleID != "LL001" {
		t.Errorf("remaining diagnostic = %q, want %q", result[0].RuleID, "LL001")
	}
}

func TestSuppressByIgnore_Wildcard(t *testing.T) {
	diags := []*diag.Diagnostic{
		{RuleID: "LL003", Pos: token.Position{Filename: "a.go", Line: 10}},
		{RuleID: "LL001", Pos: token.Position{Filename: "a.go", Line: 10}},
	}
	ignores := []IgnoreDirective{
		{RuleID: "*", Reason: "all", Pos: token.Position{Filename: "a.go", Line: 10}, Valid: true},
	}
	result := SuppressByIgnore(diags, ignores)
	if len(result) != 0 {
		t.Errorf("got %d diagnostics after wildcard suppress, want 0", len(result))
	}
}

func TestSuppressByIgnore_InvalidReported(t *testing.T) {
	diags := []*diag.Diagnostic{
		{RuleID: "LL003", Pos: token.Position{Filename: "a.go", Line: 10}},
	}
	ignores := []IgnoreDirective{
		{RuleID: "LL003", Reason: "", Pos: token.Position{Filename: "a.go", Line: 5}, Valid: false},
	}
	result := SuppressByIgnore(diags, ignores)
	if len(result) != 2 {
		t.Fatalf("got %d diagnostics, want 2 (original + invalid-ignore)", len(result))
	}
	foundInvalid := false
	for _, d := range result {
		if d.RuleID == "lurpiclint-invalid-ignore" {
			foundInvalid = true
		}
	}
	if !foundInvalid {
		t.Error("expected invalid-ignore diagnostic")
	}
}

func TestDiscover(t *testing.T) {
	// Create a temp directory with a .lurpiclint.toml file.
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, ".lurpiclint.toml")
	if err := os.WriteFile(cfgPath, []byte("[lurpiclint]\nfail_on = \"warn\"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	subdir := filepath.Join(dir, "sub")
	if err := os.Mkdir(subdir, 0755); err != nil {
		t.Fatal(err)
	}

	found, err := Discover(subdir)
	if err != nil {
		t.Fatal(err)
	}
	if found != cfgPath {
		t.Errorf("Discover() = %q, want %q", found, cfgPath)
	}
}

func TestDiscover_NotFound(t *testing.T) {
	dir := t.TempDir()
	found, err := Discover(dir)
	if err != nil {
		t.Fatal(err)
	}
	if found != "" {
		t.Errorf("Discover() = %q, want empty", found)
	}
}

func TestLoadFile(t *testing.T) {
	content := `[lurpiclint]
fail_on = "warn"

[rules.LL003]
severity = "error"

[rules.LL012]
severity = "off"

[paths]
exclude = ["vendor/**"]

[capabilities]
roots = ["marks/...", "layout/..."]
`
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, ".lurpiclint.toml")
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Lurpiclint.FailOn != "warn" {
		t.Errorf("FailOn = %q, want %q", cfg.Lurpiclint.FailOn, "warn")
	}
	if cfg.RuleSeverity("LL003") != "error" {
		t.Errorf("LL003 severity = %q, want %q", cfg.RuleSeverity("LL003"), "error")
	}
	if cfg.RuleSeverity("LL012") != "off" {
		t.Errorf("LL012 severity = %q, want %q", cfg.RuleSeverity("LL012"), "off")
	}
	if len(cfg.Paths.Exclude) != 1 || cfg.Paths.Exclude[0] != "vendor/**" {
		t.Errorf("Exclude = %v, want [vendor/**]", cfg.Paths.Exclude)
	}
	if len(cfg.Capabilities.Roots) != 2 {
		t.Errorf("Capabilities.Roots = %v, want 2 entries", cfg.Capabilities.Roots)
	}
}

func TestSuppressByBaseline_Suppresses(t *testing.T) {
	diags := []*diag.Diagnostic{
		{RuleID: "LL003", Pos: token.Position{Filename: "a.go", Line: 10}, Message: "err"},
		{RuleID: "LL001", Pos: token.Position{Filename: "a.go", Line: 20}, Message: "warn"},
	}
	baseline := &Baseline{
		Entries: []BaselineEntry{
			{RuleID: "LL003", File: "a.go", Line: 10, Message: "err"},
		},
	}
	filtered, stale := SuppressByBaseline(diags, baseline)
	if len(filtered) != 1 {
		t.Fatalf("got %d diagnostics after baseline suppress, want 1", len(filtered))
	}
	if filtered[0].RuleID != "LL001" {
		t.Errorf("remaining diagnostic = %q, want %q", filtered[0].RuleID, "LL001")
	}
	if len(stale) != 0 {
		t.Errorf("stale entries = %d, want 0", len(stale))
	}
}

func TestSuppressByBaseline_Stale(t *testing.T) {
	diags := []*diag.Diagnostic{}
	baseline := &Baseline{
		Entries: []BaselineEntry{
			{RuleID: "LL003", File: "gone.go", Line: 10, Message: "fixed"},
		},
	}
	_, stale := SuppressByBaseline(diags, baseline)
	if len(stale) != 1 {
		t.Fatalf("got %d stale entries, want 1", len(stale))
	}
	if stale[0].RuleID != "LL003" {
		t.Errorf("stale rule = %q, want %q", stale[0].RuleID, "LL003")
	}
}

func TestSuppressByBaseline_Nil(t *testing.T) {
	diags := []*diag.Diagnostic{{RuleID: "LL003"}}
	filtered, stale := SuppressByBaseline(diags, nil)
	if len(filtered) != 1 {
		t.Errorf("got %d filtered, want 1 (nil baseline should not suppress)", len(filtered))
	}
	if len(stale) != 0 {
		t.Errorf("got %d stale, want 0", len(stale))
	}
}

func TestSuppressByBaseline_NewFindingStillFails(t *testing.T) {
	diags := []*diag.Diagnostic{
		{RuleID: "LL003", Pos: token.Position{Filename: "a.go", Line: 10}, Message: "existing"},
		{RuleID: "LL001", Pos: token.Position{Filename: "a.go", Line: 30}, Message: "new"},
	}
	baseline := &Baseline{
		Entries: []BaselineEntry{
			{RuleID: "LL003", File: "a.go", Line: 10, Message: "existing"},
		},
	}
	filtered, _ := SuppressByBaseline(diags, baseline)
	if len(filtered) != 1 {
		t.Fatalf("got %d filtered, want 1 (new finding should remain)", len(filtered))
	}
	if filtered[0].RuleID != "LL001" {
		t.Errorf("remaining = %q, want %q", filtered[0].RuleID, "LL001")
	}
}
