package marks

// rtlExempt lists mark type names that are inherently symmetric or
// direction-agnostic and are therefore exempt from the requirement that
// RTL rendering differs from LTR. A mark type belongs here only when
// horizontal mirroring would produce identical pixels (e.g. centered
// circular indicators).
//
// Adding a mark to this list requires a comment explaining why it is
// symmetric. Removing a mark from this list requires that the mark's
// RTL golden test assert non-identity with the LTR golden.
//
// These entries will be folded into the stateExempt table in TestStateDiscrimination
// (marks/state_conformance_test.go, Phase 06) as "<type>/rtl" keys.
var rtlExempt = map[string]string{
	"progress_ring": "centered circular indicator with optional centered label; radial symmetry makes RTL a no-op",
	"status_light":  "small centered circular status indicator; no directional content to mirror",
}
