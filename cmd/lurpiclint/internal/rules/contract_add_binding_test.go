package rules

import (
	"path/filepath"
	"testing"

	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/internal/diag"
)

func TestLL037_FiresOnAppSideAddBinding(t *testing.T) {
	dir := ruleTestdataDir(t, "ll037_bad")
	diags := runRulesOnFixture(t, []string{"LL037"}, dir)
	if len(diags) == 0 {
		t.Fatal("expected LL037 to fire on an app-side AddBinding call")
	}
	for _, d := range diags {
		if d.RuleID != "LL037" {
			t.Fatalf("unexpected rule %s", d.RuleID)
		}
	}
}

func TestLL037_SilentInsideMarks(t *testing.T) {
	dir := ruleTestdataDir(t, "ll037_good")
	diags := runRulesOnFixture(t, []string{"LL037"}, dir)
	if len(diags) != 0 {
		for _, d := range diags {
			t.Logf("unexpected: %s:%d: %s", filepath.Base(d.Pos.Filename), d.Pos.Line, d.Message)
		}
		t.Fatalf("expected 0 LL037 diagnostics inside marks/, got %d", len(diags))
	}
}

func TestLL037_RegisteredInDefaultRegistry(t *testing.T) {
	rule := DefaultRegistry.Lookup("LL037")
	if rule == nil {
		t.Fatal("LL037 not found in DefaultRegistry — is init() missing?")
	}
	if rule.ID() != "LL037" {
		t.Errorf("rule ID = %q, want LL037", rule.ID())
	}
	if rule.DefaultSeverity() != diag.SeverityError {
		t.Errorf("LL037 DefaultSeverity = %d, want error", rule.DefaultSeverity())
	}
}
