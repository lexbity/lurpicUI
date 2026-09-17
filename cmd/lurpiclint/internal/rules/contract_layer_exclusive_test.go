package rules

import (
	"path/filepath"
	"testing"
)

func TestLL034_FiresOnGroupArrangedLayerChild(t *testing.T) {
	dir := ruleTestdataDir(t, "ll034_bad")
	diags := runRulesOnFixture(t, []string{"LL034"}, dir)
	if len(diags) == 0 {
		t.Fatal("expected LL034 to fire on a layer child listed in Children()")
	}
	for _, d := range diags {
		if d.RuleID != "LL034" {
			t.Fatalf("unexpected rule %s", d.RuleID)
		}
	}
}

func TestLL034_SilentWhenLayerChildExclusive(t *testing.T) {
	dir := ruleTestdataDir(t, "ll034_good")
	diags := runRulesOnFixture(t, []string{"LL034"}, dir)
	if len(diags) != 0 {
		for _, d := range diags {
			t.Logf("unexpected: %s:%d: %s", filepath.Base(d.Pos.Filename), d.Pos.Line, d.Message)
		}
		t.Fatalf("expected 0 LL034 diagnostics on exclusive fixture, got %d", len(diags))
	}
}

func TestLL035_FiresOnStoreWriteInOnProject(t *testing.T) {
	dir := ruleTestdataDir(t, "ll035_bad")
	diags := runRulesOnFixture(t, []string{"LL035"}, dir)
	if len(diags) == 0 {
		t.Fatal("expected LL035 to fire on a store write inside OnProject")
	}
}

func TestLL035_SilentOnPureProjection(t *testing.T) {
	dir := ruleTestdataDir(t, "ll035_good")
	diags := runRulesOnFixture(t, []string{"LL035"}, dir)
	if len(diags) != 0 {
		t.Fatalf("expected 0 LL035 diagnostics on pure fixture, got %d", len(diags))
	}
}

func TestLL034_RegisteredInDefaultRegistry(t *testing.T) {
	if DefaultRegistry.Lookup("LL034") == nil {
		t.Fatal("LL034 not found in DefaultRegistry — is init() missing?")
	}
}

func TestLL035_RegisteredInDefaultRegistry(t *testing.T) {
	if DefaultRegistry.Lookup("LL035") == nil {
		t.Fatal("LL035 not found in DefaultRegistry — is init() missing?")
	}
}
