package feedback

import (
	"image/color"
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/store"
)

// The integration tests prove the outside-click dismissal junction for the
// Dialog mark through the real runtime path: a pointer press outside the
// overlay's bounds lands on the base layer, dismissalEventsForPointerPresses
// emits a DismissEvent to the dismissal-enabled layer the dialog is mounted in,
// and the dialog's OnDismiss closes it.
//
// Mounting: the dialog is mounted as a layered overlay via facet.AttachLayer
// (the framework's layered-overlay API) and bound to the registry's
// dismissal-enabled layer at runtime.

func TestDialogIntegration_ClickOutsideDismisses(t *testing.T) {
	open := store.NewValueStore(true)
	dlg := NewDialog("Confirm", "This action cannot be undone.", nil, open)

	root := newOverlayRoot()
	facet.AttachLayer(root, dlg, facet.LayerAttachment{ZPriority: 100})

	h, modalID := newOverlayHarness(t, root)
	h.Runtime().UpdateChildAttachment(dlg, facet.Attachment{LayerID: modalID})
	testkit.Warmup(h)

	sb := dlg.cachedSurfaceBounds
	if sb.IsEmpty() {
		t.Fatal("expected the dialog surface to be arranged after warmup")
	}
	px := int(sb.Min.X + sb.Width()/2)
	py := int(sb.Min.Y + sb.Height()/2)
	before := h.Surface().PixelAt(px, py)

	// Click far outside the dialog's bounds: the press lands on the root's
	// base layer, so the dialog's own pointer handler is not involved — the
	// dismissal comes from the runtime's dismissal events.
	testkit.DriveClick(h, 300, 180)

	if open.Get() {
		t.Fatal("expected the outside click to dismiss the dialog")
	}
	after := h.Surface().PixelAt(px, py)
	if before == after {
		t.Fatalf("expected the dismissed dialog's surface to leave the frame: before=%#v after=%#v", before, after)
	}
	// The overlay's render commands are gone: the surface region shows the
	// root background again.
	testkit.AssertPixelColor(t, h.Surface(), px, py, color.RGBA{R: 51, G: 102, B: 204, A: 255}, 2)
}

func TestDialogIntegration_PlainChildDoesNotDismiss(t *testing.T) {
	open := store.NewValueStore(true)
	dlg := NewDialog("Confirm", "Body", nil, open)

	root := newOverlayRoot()
	h, _ := newOverlayHarness(t, root)

	// Deliberate miswire: mount the dialog as a plain child with AddFacet and
	// no layer attachment — no facet.AttachLayer ZPriority, no LayerID. The
	// runtime never mounts it into a dismissal-enabled layer, so an outside
	// click cannot emit a DismissEvent.
	h.Runtime().AddFacet(root, dlg, facet.Attachment{})

	testkit.Warmup(h)
	testkit.DriveClick(h, 300, 180)

	if !open.Get() {
		t.Fatal("expected a plain (non-layer) dialog child to survive an outside click")
	}
}
