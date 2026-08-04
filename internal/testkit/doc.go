// Package testkit provides headless helpers for engine tests.
//
// # Interaction tests and the warmup-frame contract
//
// The runtime routes platform input against the PREVIOUS frame's hit map. On
// the very first frame after NewHarness the hit map is nil, so pointer events
// injected before any RunFrame are silently dropped. Interaction tests MUST
// use the Drive* helpers (DriveClick, DriveKeyPress, DriveKeyRelease,
// DriveType, DriveDrag, DriveScroll) which run a warmup frame first, then one
// frame per injected event. Tests that inject raw events via InjectEvent MUST
// call Warmup (or RunFrame) before injecting pointer events. See drive.go.
//
// A mark mounted as the harness root fills the window. To mount a mark as a
// non-root child with known bounds (clipping-dependent marks), compose it with
// the framework's layout containers (layout.NewSizedBox, layout.NewStackLayout,
// layout.NewColumnLayout) rather than hand-rolling a LayoutRole — LL003 blocks
// hand-rolled child-arranging containers outside layout/, marks/, and runtime/.
//
// # Golden discrimination contract
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
