package studio

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/marks/primitive"
)

// TestInspector_derived_binding_reprojects_within_two_frames proves RX-1 FR-2 /
// AC-3 at the shell level: a FromDerived-bound label updates within two frames
// of an upstream store write, with no user-code Get() flush anywhere.
//
// On pre-P1 code the FromDerived binding subscribed to Derived.OnChange, which
// emits only inside Get() — and nobody calls Get() unless the facet re-projects,
// which only happens for dirty facets. The write therefore never invalidated the
// bound facet and its projection cache kept serving stale output (A-6). P1
// (FR-2) subscribes the binding to the eager Derived.OnInvalidated signal and
// forces the recompute in the signal-delivery phase, so the facet enters the
// next frame's dirty set with a freshly recomputed value.
//
// The asserted mark is the status bar caption: the shell's attached
// FromDerived-bound label. The inspector's title/desc/count texts are Card grid
// children — self-projected content that is not a facet-tree child — so their
// re-measure + re-arrange is the FR-3 slice P3 concern. Re-measure through
// ancestor policies is likewise FR-3; this test pins the P1 half of the chain:
// the binding fires, and the value the projection will read is already fresh.
func TestInspector_derived_binding_reprojects_within_two_frames(t *testing.T) {
	root, h := newShell(t, 1280, 800)
	h.RunFrames(2) // settle to steady state

	captionText := root.StatusBar().Caption().(*primitive.Text)
	captionID := captionText.Base().ID()

	if captionText.Content.Get() != exhibitTitle(ExhibitRealtime) {
		t.Fatalf("precondition: caption = %q, want %q",
			captionText.Content.Get(), exhibitTitle(ExhibitRealtime))
	}

	// Post-attach store write. On pre-P1 code the FromDerived binding never
	// fires (A-6): the caption facet stays out of the frame's dirty set and the
	// derived's recompute never runs.
	root.Shell().ActiveExhibit.Set(ExhibitLayers)

	// The write must be delivered in the signal phase and invalidate the bound
	// facet before projection within the same frame (AC-3: "within 2 frames").
	h.RunFrame()

	snap := h.Runtime().LastDirtySnapshot()
	if flags := snap[captionID]; flags&facet.DirtyProjection == 0 {
		t.Fatalf("caption facet was not invalidated within one frame of the ActiveExhibit write "+
			"(dirty=%v) — the FromDerived binding stalled (A-6)", flags)
	}

	// The recompute happened in the signal-delivery phase, not lazily at
	// projection time: the value the projection reads is already fresh.
	if got := captionText.Content.Get(); got != exhibitTitle(ExhibitLayers) {
		t.Fatalf("caption value = %q, want %q", got, exhibitTitle(ExhibitLayers))
	}

	// The frame chain must settle to silence (NFR-5 frame discipline). The
	// exhibit switch also re-lays the chart y-axis (its scale recomputes —
	// RX-1 F-dirtylayout-routing routes the axis's DirtyLayout), which takes
	// one extra frame, so settle until quiet.
	for i := 0; i < 4; i++ {
		h.RunFrame()
		if snap := h.Runtime().LastDirtySnapshot(); len(snap) == 0 {
			return
		}
	}
	if snap := h.Runtime().LastDirtySnapshot(); len(snap) != 0 {
		t.Fatalf("expected a quiet frame after the switch settled, got dirty facets=%v", snap)
	}
}
