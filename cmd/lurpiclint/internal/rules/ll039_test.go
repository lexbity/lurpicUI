package rules

import (
	"testing"
)

func TestLL039_OnBadFixture(t *testing.T) {
	dir := ruleTestdataDir(t, "ll039", "bad", "marks", "action")
	diags := runRulesOnFixture(t, []string{"LL039"}, dir)
	if len(diags) != 1 {
		t.Fatalf("expected 1 LL039 diagnostic (one duplicated literal, reported once), got %d", len(diags))
	}
	for _, d := range diags {
		if d.RuleID != "LL039" {
			t.Errorf("unexpected rule %q, want LL039", d.RuleID)
		}
	}
}

func TestLL039_OnGoodFixture(t *testing.T) {
	dir := ruleTestdataDir(t, "ll039", "good", "marks", "action")
	diags := runRulesOnFixture(t, []string{"LL039"}, dir)
	if len(diags) != 0 {
		t.Errorf("expected 0 diagnostics on good fixture, got %d", len(diags))
		for _, d := range diags {
			t.Logf("  unexpected: %s", d.Message)
		}
	}
}
