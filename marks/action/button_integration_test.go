package action

import (
	"sync/atomic"
	"testing"

	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uiinput"
)

// The integration tests prove the event -> routing -> handler -> state ->
// pixels junction against a real production mark (Button) through the runtime,
// using the testkit Drive* helpers. The handler-fire assertion is load-bearing;
// the pixel assertion confirms the press/release visibly changed the frame.
//
// Mounting paths exercised (see testkit/doc.go "three blessed paths"):
//   - TestButtonIntegration_*: Q7 path 1 — the mark is the harness root.
//   - TestButtonIntegration_SizedBoxNonRoot: Q7 path 2 — the mark is a non-root
//     child of a blessed layout container (layout.NewSizedBox).

func newIntegrationButton(t *testing.T, activations *int32) *Button {
	t.Helper()
	btn := NewButton(marks.Const("OK"), marks.Const(uiinput.ButtonFilled))
	btn.Activated.Subscribe(func(signal.Unit) {
		atomic.AddInt32(activations, 1)
	})
	return btn
}

func TestButtonIntegration_ClickActivatesHandler(t *testing.T) {
	var activations int32
	btn := newIntegrationButton(t, &activations)

	// Q7 path 1: mount the mark directly as the harness root. It fills the window.
	h := testkit.NewHarness(t, testkit.StandardHarnessConfig(t, 120, 48), btn)
	testkit.Warmup(h)

	b := btn.Layout.ArrangedBounds
	if b.IsEmpty() {
		t.Fatal("expected button to be arranged to the window after warmup")
	}
	cx := b.Min.X + b.Width()/2
	cy := b.Min.Y + b.Height()/2

	testkit.DriveClick(h, cx, cy)

	if got := atomic.LoadInt32(&activations); got != 1 {
		t.Fatalf("expected 1 activation after DriveClick at the button center, got %d", got)
	}
}

func TestButtonIntegration_ClickOutsideDoesNotActivate(t *testing.T) {
	var activations int32
	btn := newIntegrationButton(t, &activations)

	h := testkit.NewHarness(t, testkit.StandardHarnessConfig(t, 120, 48), btn)
	testkit.Warmup(h)

	// Click far outside the window; nothing should fire.
	testkit.DriveClick(h, 999, 999)

	if got := atomic.LoadInt32(&activations); got != 0 {
		t.Fatalf("expected 0 activations after DriveClick outside the window, got %d", got)
	}
}

func TestButtonIntegration_PressAndReleaseChangePixels(t *testing.T) {
	var activations int32
	btn := newIntegrationButton(t, &activations)

	h := testkit.NewHarness(t, testkit.StandardHarnessConfig(t, 120, 48), btn)
	testkit.Warmup(h)

	b := btn.Layout.ArrangedBounds
	cx := int(b.Min.X + b.Width()/2)
	cy := int(b.Min.Y + b.Height()/2)

	resting := h.Surface().PixelAt(cx, cy)

	h.InjectEvent(testkit.PointerPress(float32(cx), float32(cy), platform.PointerLeft))
	h.RunFrame()
	pressed := h.Surface().PixelAt(cx, cy)

	h.InjectEvent(testkit.PointerRelease(float32(cx), float32(cy), platform.PointerLeft))
	h.RunFrame()
	released := h.Surface().PixelAt(cx, cy)

	if got := atomic.LoadInt32(&activations); got != 1 {
		t.Fatalf("expected 1 activation after press+release, got %d", got)
	}
	if pressed == resting {
		t.Fatalf("expected the press to change the button face pixel: resting=%#v pressed=%#v", resting, pressed)
	}
	if released != resting {
		t.Fatalf("expected the release to restore the resting pixel: resting=%#v released=%#v", resting, released)
	}
}

func TestButtonIntegration_SizedBoxNonRootRoutesClick(t *testing.T) {
	var activations int32
	btn := newIntegrationButton(t, &activations)

	// Q7 path 2: mount the mark as a non-root child of a blessed layout
	// container. The SizedBox hosts the button; the button fills its bounds.
	host := layout.NewSizedBox(120, 48, btn)

	h := testkit.NewHarness(t, testkit.StandardHarnessConfig(t, 120, 48), host)
	testkit.Warmup(h)

	box := host.Base().LayoutRole().ArrangedBounds
	if box.IsEmpty() {
		t.Fatal("expected sized box to be arranged after warmup")
	}
	cx := box.Min.X + box.Width()/2
	cy := box.Min.Y + box.Height()/2

	testkit.DriveClick(h, cx, cy)

	if got := atomic.LoadInt32(&activations); got != 1 {
		t.Fatalf("expected 1 activation via SizedBox-mounted button, got %d", got)
	}
	if btn.Layout.ArrangedBounds.IsEmpty() {
		t.Fatal("expected the button (child of SizedBox) to be arranged")
	}
}
