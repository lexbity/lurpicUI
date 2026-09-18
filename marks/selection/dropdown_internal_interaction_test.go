package selection

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/store"
)

// TestDropdownSelect_popupItemClickSelects pins the composite
// internal-interaction contract (RX-1 F-e6-internal): a dropdown's popup
// options are internal sub-facets (the listbox layer), but a real pointer
// click on an option is driveable from outside the composite — the runtime
// routes the hit to the dropdown's listbox layer and the dropdown selects the
// option through its Value store.
func TestDropdownSelect_popupItemClickSelects(t *testing.T) {
	value := store.NewValueStore("")
	sel := NewDropdownSelect("City", []DropdownOption{
		{Value: "syd", Label: "Sydney"},
		{Value: "canb", Label: "Canberra"},
		{Value: "melb", Label: "Melbourne"},
	}, value)
	sel.Placeholder = marks.Const("Pick a city")

	h := testkit.NewHarness(t, testkit.StandardHarnessConfig(t, 300, 200), sel)
	testkit.Warmup(h)

	// Click the trigger to open the listbox.
	trigger := sel.cachedTriggerBounds
	if trigger.IsEmpty() {
		t.Fatal("trigger has no arranged bounds")
	}
	tx := trigger.Min.X + trigger.Width()*0.5
	ty := trigger.Min.Y + trigger.Height()*0.5
	testkit.DriveClick(h, tx, ty)
	h.RunFrames(3)
	if !sel.open {
		t.Fatal("trigger click did not open the listbox")
	}

	// Click the second option.
	optionRects := sel.ensureOptionRects()
	if len(optionRects) < 2 {
		t.Fatalf("expected >= 2 option rects, got %d", len(optionRects))
	}
	option := optionRects[1]
	ox := option.Min.X + option.Width()*0.5
	oy := option.Min.Y + option.Height()*0.5
	testkit.DriveClick(h, ox, oy)
	h.RunFrame()

	if got := value.Get(); got != "canb" {
		t.Fatalf("after option click value = %q, want canb (F-e6-internal)", got)
	}
	_ = gfx.Rect{}
}
