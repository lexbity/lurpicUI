package runtime

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/internal/membudget"
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
