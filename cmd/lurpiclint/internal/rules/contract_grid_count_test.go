package rules

import (
	"path/filepath"
	"testing"
)

func TestLL036_FiresOnSuspiciousGridCount(t *testing.T) {
	dir := ruleTestdataDir(t, "ll036_bad")
	diags := runRulesOnFixture(t, []string{"LL036"}, dir)
	if len(diags) == 0 {
		t.Fatal("expected LL036 to fire on a GridRows count above 16")
	}
	for _, d := range diags {
		if d.RuleID != "LL036" {
			t.Fatalf("unexpected rule %s", d.RuleID)
		}
	}
}

func TestLL036_SilentOnReasonableGridCount(t *testing.T) {
	dir := ruleTestdataDir(t, "ll036_good")
	diags := runRulesOnFixture(t, []string{"LL036"}, dir)
	if len(diags) != 0 {
		for _, d := range diags {
			t.Logf("unexpected: %s:%d: %s", filepath.Base(d.Pos.Filename), d.Pos.Line, d.Message)
		}
		t.Fatalf("expected 0 LL036 diagnostics on a small grid count, got %d", len(diags))
	}
}
