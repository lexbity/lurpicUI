package selection

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/store"
)

// TestValueMarks_nilStoreConstruction pins the RX-2 Q2 doctrine for the
// selection family: a nil value store means the mark creates an internal
// store and owns its truth — construction renders, Store() exposes the live
// store, and a user-edit write-back lands in it. Before the value contract a
// nil store silently disabled every subscription (the B-2 bug class).
func TestValueMarks_nilStoreConstruction(t *testing.T) {
	slider := NewSlider("Opacity", 0, 100, 5, nil)
	if slider.Store() == nil {
		t.Fatal("slider: nil store did not create an internal store")
	}
	if got := slider.Store().Get(); got != 0 {
		t.Fatalf("slider: internal store seeded %v, want the normalized minimum 0", got)
	}
	slider.SetValue(40)
	if slider.Store().Get() != 40 {
		t.Fatalf("slider: user edit did not write the internal store: %v", slider.Store().Get())
	}

	sw := NewSwitch("Live", nil)
	if sw.Store() == nil {
		t.Fatal("switch: nil store did not create an internal store")
	}
	sw.SetChecked(true)
	if !sw.Store().Get() {
		t.Fatal("switch: user edit did not write the internal store")
	}

	cb := NewCheckbox("Grid", nil)
	if cb.Store() == nil {
		t.Fatal("checkbox: nil store did not create an internal store")
	}
	cb.SetState(CheckboxStateOn)
	if cb.Store().Get() != CheckboxStateOn {
		t.Fatal("checkbox: user edit did not write the internal store")
	}

	radio := NewRadioGroup("Chart", []RadioOption{{Value: "a", Label: "A"}}, nil)
	if radio.Store() == nil {
		t.Fatal("radio_group: nil store did not create an internal store")
	}
	radio.SetValue("a")
	if radio.Store().Get() != "a" {
		t.Fatal("radio_group: user edit did not write the internal store")
	}

	dd := NewDropdownSelect("Agg", []DropdownOption{{Value: "d", Label: "D"}}, nil)
	if dd.Store() == nil {
		t.Fatal("dropdown_select: nil store did not create an internal store")
	}
	dd.chooseIndex(0)
	if dd.Store().Get() != "d" {
		t.Fatal("dropdown_select: user edit did not write the internal store")
	}

	dial := NewTurnDial("Smooth", 10, 90, 1, nil)
	if dial.Store() == nil {
		t.Fatal("turn_dial: nil store did not create an internal store")
	}
	if got := dial.Store().Get(); got != 10 {
		t.Fatalf("turn_dial: internal store seeded %v, want the minimum 10", got)
	}
	dial.SetValue(50)
	if dial.Store().Get() != 50 {
		t.Fatal("turn_dial: user edit did not write the internal store")
	}

	bg := NewButtonGroup("Range", []ButtonGroupOption{{Key: "d", Label: "D"}}, nil)
	if bg.Store() == nil {
		t.Fatal("button_group: nil store did not create an internal store")
	}
	bg.SetSelectedKeys("d")
	if got := bg.Store().Get(); len(got) != 1 || got[0] != "d" {
		t.Fatalf("button_group: user edit did not write the internal store: %v", got)
	}

	// Every internal store is the mark's own truth: writes through the store
	// are visible to the mark (injected stores are bound, never copied).
	sw.Store().Set(false)
	if sw.isChecked() {
		t.Fatal("switch: store write not visible through the mark")
	}
}

// TestValueMarks_injectedStoreBoundNotCopied pins the injection half of the
// doctrine: an injected store is the live truth, not a copied seed.
func TestValueMarks_injectedStoreBoundNotCopied(t *testing.T) {
	s := store.NewValueStore(25.0)
	slider := NewSlider("Opacity", 0, 100, 5, s)
	if slider.Store() != s {
		t.Fatal("slider: Store() must return the injected store itself")
	}
	s.Set(75)
	if got := slider.currentValue(); got != 75 {
		t.Fatalf("slider: injected store write not visible: %v", got)
	}
}
