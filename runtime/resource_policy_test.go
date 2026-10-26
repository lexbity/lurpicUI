package runtime

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/internal/membudget"
	"codeburg.org/lexbit/lurpicui/text"
)

func TestDefaultResourceProfiles(t *testing.T) {
	for _, profile := range []membudget.Profile{membudget.Compact, membudget.Standard} {
		policy := DefaultResourcePolicy(profile)
		if err := policy.Validate(); err != nil {
			t.Fatalf("%s: %v", profile, err)
		}
	}
}

func TestResourcePolicyRejectsUnreservedTextGrowth(t *testing.T) {
	policy := DefaultResourcePolicy(membudget.Compact)
	policy.TextLimits.MaxSourceBytes++
	if err := policy.Validate(); err == nil {
		t.Fatal("text limit exceeded reservation")
	}
	policy = DefaultResourcePolicy(membudget.Compact)
	policy.TextLimits.MaxLayouts++
	if err := policy.Validate(); err == nil {
		t.Fatal("metadata limit exceeded compact profile")
	}
}

func TestResourcePolicyRejectsUninitializedPlan(t *testing.T) {
	if err := (ResourcePolicy{}).Validate(); err == nil {
		t.Fatal("zero policy was accepted as an implicit unlimited/default plan")
	}
}

func TestRuntimeRejectsFontWalletOutsideResourcePlan(t *testing.T) {
	policy := DefaultResourcePolicy(membudget.Compact)
	wallet, err := membudget.NewWallet(policy.Plan.Caps[membudget.Text])
	if err != nil {
		t.Fatal(err)
	}
	registry, err := text.NewFontRegistry(wallet, policy.TextLimits.MaxFontFaces)
	if err != nil {
		t.Fatal(err)
	}
	defer registry.Close()
	config := DefaultConfig()
	config.FontRegistry = registry
	if _, err := New(config, nil, nil, nil, nil); err == nil {
		t.Fatal("runtime accepted a compact registry under the standard plan")
	}
}
