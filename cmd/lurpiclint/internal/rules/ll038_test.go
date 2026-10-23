package rules

import (
	"testing"
)

func TestLL038_OnBadFixture(t *testing.T) {
	dir := ruleTestdataDir(t, "ll038", "bad", "marks", "action")
	diags := runRulesOnFixture(t, []string{"LL038"}, dir)
	if len(diags) != 3 {
		t.Fatalf("expected 3 LL038 diagnostics (2 named interfaces + 1 inline assertion), got %d", len(diags))
	}
	for _, d := range diags {
		if d.RuleID != "LL038" {
			t.Errorf("unexpected rule %q, want LL038", d.RuleID)
		}
	}
}

func TestLL038_OnGoodFixture(t *testing.T) {
	dir := ruleTestdataDir(t, "ll038", "good", "marks", "action")
	diags := runRulesOnFixture(t, []string{"LL038"}, dir)
	if len(diags) != 0 {
		t.Errorf("expected 0 diagnostics on good fixture, got %d", len(diags))
		for _, d := range diags {
			t.Logf("  unexpected: %s", d.Message)
		}
	}
}
