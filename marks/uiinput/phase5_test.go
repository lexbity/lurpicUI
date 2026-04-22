package uiinput

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/store"
)

func TestPhase5ButtonCheckboxRadioSelectSliderSwitchTextInputGeometry(t *testing.T) {
	if got := (&Button{}).bounds(); got.Width() != 96 || got.Height() != 36 {
		t.Fatalf("button bounds = %#v", got)
	}
	if got := (&Checkbox{}).bounds(); got.Width() != 28 || got.Height() != 28 {
		t.Fatalf("checkbox bounds = %#v", got)
	}
	if got := (&RadioGroup{Options: []RadioOption{{Key: "a"}, {Key: "b"}}}).bounds(); got.Width() != 160 || got.Height() != 56 {
		t.Fatalf("radiogroup bounds = %#v", got)
	}
	if got := (&Select{Options: []SelectOption{{Key: "a"}, {Key: "b"}}}).bounds(); got.Width() != 180 || got.Height() != 36 {
		t.Fatalf("select bounds = %#v", got)
	}
	if got := (&Select{Options: []SelectOption{{Key: "a"}, {Key: "b"}}}).popupBounds(); got.Width() != 180 || got.Height() != 56 {
		t.Fatalf("select popup bounds = %#v", got)
	}
	if got := (&Slider{}).bounds(); got.Width() != 240 || got.Height() != 28 {
		t.Fatalf("horizontal slider bounds = %#v", got)
	}
	if got := (&Slider{Orientation: SliderVertical}).bounds(); got.Width() != 28 || got.Height() != 200 {
		t.Fatalf("vertical slider bounds = %#v", got)
	}
	if got := (&Switch{}).bounds(); got.Width() != 44 || got.Height() != 28 {
		t.Fatalf("switch bounds = %#v", got)
	}
	if got := (&TextInput{Value: store.NewBinding("")}).bounds(); got.Width() != 280 || got.Height() != 36 {
		t.Fatalf("textinput bounds = %#v", got)
	}
	if got := (&TextInput{Value: store.NewBinding(""), Multiline: true}).bounds(); got.Width() != 280 || got.Height() != 120 {
		t.Fatalf("multiline textinput bounds = %#v", got)
	}
}
