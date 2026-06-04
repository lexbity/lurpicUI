package marks

import "testing"

// rtlExempt lists mark type names that are inherently symmetric or
// direction-agnostic and are therefore exempt from the requirement that
// RTL rendering differs from LTR. A mark type belongs here only when
// horizontal mirroring would produce identical pixels (e.g. centered
// circular indicators).
//
// Adding a mark to this list requires a comment explaining why it is
// symmetric. Removing a mark from this list requires that the mark's
// RTL golden test assert non-identity with the LTR golden.
var rtlExempt = map[string]string{
	"progress_ring": "centered circular indicator with optional centered label; radial symmetry makes RTL a no-op",
	"status_light":  "small centered circular status indicator; no directional content to mirror",
}

// TestRTLExempt_documented verifies that the rtlExempt table is
// non-empty and contains at least the symmetric marks we know about.
// This is a documentation assertion — it fails if the table is
// accidentally emptied.
func TestRTLExempt_documented(t *testing.T) {
	if len(rtlExempt) == 0 {
		t.Fatal("rtlExempt table is empty — if all marks now support RTL, great, but this should be intentional")
	}
	for name, reason := range rtlExempt {
		if reason == "" {
			t.Errorf("rtlExempt[%q] has empty reason", name)
		}
	}
}
