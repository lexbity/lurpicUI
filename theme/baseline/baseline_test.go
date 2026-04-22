package baseline

import "testing"

func TestDefault_matchesPlanTables(t *testing.T) {
	b := Default()

	if got := b.UIInput.Button.Height.Regular; got != 40 {
		t.Fatalf("button regular height = %v", got)
	}
	if got := b.UIInput.Slider.TrackThickness.Touchspread; got != 6 {
		t.Fatalf("slider touchspread thickness = %v", got)
	}
	if got := b.UINav.Menu.RowHeight.Compact; got != 28 {
		t.Fatalf("menu compact row height = %v", got)
	}
	if got := b.UINotification.Dialog.MaxWidth.Regular; got != 640 {
		t.Fatalf("dialog regular max width = %v", got)
	}
}
