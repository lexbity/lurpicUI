package selection

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/store"
)

// The integration tests prove the click -> routing -> handler -> store ->
// pixels junction for the Checkbox mark through the runtime (Q7 path 1:
// mark-as-root). The store-mutation assertion is load-bearing; the pixel
// assertion confirms the checked control visibly changed the frame.

// newCheckboxIntegration builds a mounted Checkbox harness, runs the warmup
// frame, and returns the control box for click-target computation.
func newCheckboxIntegration(t *testing.T, value *store.ValueStore[CheckboxState]) (*testkit.Harness, *Checkbox, gfx.Rect) {
	t.Helper()
	cb := NewCheckbox("Enable notifications", value)
	h := testkit.NewStandardHarness(t, 200, 60, cb)
	testkit.Warmup(h)
	ctrl := cb.cachedControlBounds
	if ctrl.IsEmpty() {
		t.Fatal("expected control bounds after warmup")
	}
	return h, cb, ctrl
}

func TestCheckboxIntegration_ClickTogglesStore(t *testing.T) {
	value := store.NewValueStore(CheckboxStateOff)
	h, _, ctrl := newCheckboxIntegration(t, value)

	cx := ctrl.Min.X + ctrl.Width()/2
	cy := ctrl.Min.Y + ctrl.Height()/2

	testkit.DriveClick(h, cx, cy)
	if got := value.Get(); got != CheckboxStateOn {
		t.Fatalf("expected the first click to toggle to CheckboxStateOn, got %v", got)
	}

	testkit.DriveClick(h, cx, cy)
	if got := value.Get(); got != CheckboxStateOff {
		t.Fatalf("expected the second click to toggle back to CheckboxStateOff, got %v", got)
	}
}

func TestCheckboxIntegration_ToggleChangesPixels(t *testing.T) {
	value := store.NewValueStore(CheckboxStateOff)
	h, _, ctrl := newCheckboxIntegration(t, value)

	// Sample inside the control box but away from the white checkmark that
	// crosses its center, so the filled/empty states are distinguishable.
	px := int(ctrl.Min.X + ctrl.Width()*0.3)
	py := int(ctrl.Min.Y + ctrl.Height()*0.3)

	offPixel := h.Surface().PixelAt(px, py)

	testkit.DriveClick(h, float32(px), float32(py))

	if got := value.Get(); got != CheckboxStateOn {
		t.Fatalf("expected CheckboxStateOn after the click, got %v", got)
	}
	onPixel := h.Surface().PixelAt(px, py)
	if onPixel == offPixel {
		t.Fatalf("expected the checked control to change the pixel at (%d,%d): off=%#v on=%#v", px, py, offPixel, onPixel)
	}
}

func TestCheckboxIntegration_RapidToggleSequence(t *testing.T) {
	value := store.NewValueStore(CheckboxStateOff)
	h, _, ctrl := newCheckboxIntegration(t, value)

	cx := ctrl.Min.X + ctrl.Width()/2
	cy := ctrl.Min.Y + ctrl.Height()/2

	// Drive five clicks back-to-back and record the state after each. The odd
	// count must end on CheckboxStateOn and no click may be dropped.
	trace := make([]CheckboxState, 0, 5)
	for i := 0; i < 5; i++ {
		testkit.DriveClick(h, cx, cy)
		trace = append(trace, value.Get())
	}

	want := []CheckboxState{CheckboxStateOn, CheckboxStateOff, CheckboxStateOn, CheckboxStateOff, CheckboxStateOn}
	if len(trace) != len(want) {
		t.Fatalf("trace length = %d, want %d", len(trace), len(want))
	}
	for i := range want {
		if trace[i] != want[i] {
			t.Fatalf("trace[%d] = %v, want %v (full trace %v)", i, trace[i], want[i], trace)
		}
	}
	if got := value.Get(); got != CheckboxStateOn {
		t.Fatalf("expected the fifth click to leave the checkbox on, got %v", got)
	}
}
