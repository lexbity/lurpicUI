package studio

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/platform"
)

// newStudioHarness builds a harness with the studio layer registry and theme
// resolver (the demo's runtime configuration).
func newStudioHarness(t *testing.T, w, h int, root facet.FacetImpl) *testkit.Harness {
	t.Helper()
	reg, err := StudioLayerRegistry()
	if err != nil {
		t.Fatalf("layer registry: %v", err)
	}
	cfg := testkit.StandardHarnessConfig(t, w, h)
	cfg.LayerRegistry = reg
	cfg.ThemeResolver = StudioThemeContext().Resolver
	harness := testkit.NewHarness(t, cfg, root)
	harness.RunFrame()
	return harness
}

// TestE2_blockingAndPassThrough asserts the FR-layers contracts with real
// assertions instead of a probe:
//   - the base-layer control is hit at its center;
//   - the tooltip layer (HitPassThrough) never steals the hit while shown and
//     the control stays reachable when it is hidden;
//   - with the modal closed the invisible scrim does not block: a press
//     reaches the control (regression: bounds-derived hit regions used to let
//     the closed scrim swallow the hit);
//   - opening the modal mounts the scrim on the block-below layer so the same
//     press is consumed by the scrim;
//   - closing the modal restores pass-through.
func TestE2_blockingAndPassThrough(t *testing.T) {
	themeCtx := StudioThemeContext()
	reg, _ := StudioLayerRegistry()
	ids := studioLayersFrom(reg)
	e2 := NewLayersFacet(testkit.TestFontRegistry(t), themeCtx, ids)
	h := newStudioHarness(t, 800, 600, e2)
	h.RunFrame()

	// The control's center: E2 arranges it in the middle of the content area.
	ctrl := e2.control.layout.ArrangedBounds
	if ctrl.IsEmpty() {
		t.Fatalf("control has no arranged bounds: %v", ctrl)
	}
	pt := gfx.Point{X: ctrl.Min.X + ctrl.Width()*0.5, Y: ctrl.Min.Y + ctrl.Height()*0.5}

	if got := e2.control.Base().HitRole().OnHitTest(pt); !got.Hit {
		t.Fatalf("control declares no hit at its center %v", pt)
	}
	controlID := e2.control.Base().ID()

	// Tooltip layer (HitPassThrough): the visible tooltip must not steal the
	// hit — the base-layer control is still hit.
	if got := h.Runtime().HitTest(pt); got != controlID {
		t.Fatalf("tooltip shown: hit=%d want control %d", got, controlID)
	}

	// Hiding the tooltip unmounts it; the control stays hit.
	e2.tooltipOn.Set(false)
	h.RunFrame()
	h.RunFrame()
	if got := h.Runtime().HitTest(pt); got != controlID {
		t.Fatalf("tooltip hidden: hit=%d want control %d", got, controlID)
	}

	// Closed modal: the invisible scrim must not block — a press reaches the
	// control (the invisible-scrim regression).
	e2.tooltipOn.Set(true)
	h.RunFrame()
	h.RunFrame()
	before := e2.control.presses
	if got := h.Runtime().HitTest(pt); got != controlID {
		t.Fatalf("modal closed: hit=%d want control %d", got, controlID)
	}
	h.InjectEvent(platform.EventPointer{Kind: platform.PointerPress, Position: pt, Button: platform.PointerLeft})
	h.RunFrame()
	if e2.control.presses != before+1 {
		t.Fatalf("modal closed: press not delivered to control (presses=%d want %d)", e2.control.presses, before+1)
	}

	// Open modal: the scrim layer (HitBlockBelow) now covers the control.
	e2.modalOpen.Set(true)
	h.RunFrame()
	h.RunFrame()
	scrimID := e2.scrim.Base().ID()
	if got := h.Runtime().HitTest(pt); got != scrimID {
		t.Fatalf("modal open: hit=%d want scrim %d", got, scrimID)
	}
	before = e2.control.presses
	h.InjectEvent(platform.EventPointer{Kind: platform.PointerPress, Position: pt, Button: platform.PointerLeft})
	h.RunFrame()
	if e2.control.presses != before {
		t.Fatalf("modal open: press reached the control (presses=%d want %d)", e2.control.presses, before)
	}

	// Closing the modal restores pass-through.
	e2.modalOpen.Set(false)
	h.RunFrame()
	h.RunFrame()
	if got := h.Runtime().HitTest(pt); got != controlID {
		t.Fatalf("modal closed again: hit=%d want control %d", got, controlID)
	}
}
