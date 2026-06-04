// Package testkit provides headless helpers for engine tests.
//
// Golden discrimination contract
//
// Every {mark}_{state} golden (state ∈ rtl, focused, hovered, pressed,
// selected, open, disabled, compact, comfortable, high_contrast, dark,
// skeuomorphic, mixed) must either differ from {mark}_default beyond
// tolerance, or appear in a typed exempt registry with a one-line
// justification.
//
// The canonical idiom for RTL and variant-state testing is:
//
//	ltr, rtl := testkit.RenderRTLPair(t, func(t testing.TB, dir facet.WritingDirection) *testkit.MemorySurface {
//	    return renderMark(ctx.WithWritingDirection(dir))
//	})
//	testkit.AssertGoldenPair(t, ltr, rtl, "mark_name")
//
// AssertGoldenPair asserts that:
//   - the LTR surface matches <baseName>_default
//   - the RTL surface matches <baseName>_rtl
//   - the two surfaces differ from each other (the discrimination gate)
//
// For variant states where the golden is generated with a single call,
// use the available AssertDiffers when comparing variant vs default:
//
//	testkit.AssertDiffers(t, variant, def, "mark_name")
//
// See also: AssertGolden, AssertGoldenPair, AssertDiffers, RenderRTLPair.
package testkit
