package widgets

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/voiceux"
	"codeburg.org/lexbit/voicedsp"
)

func TestParameterSlider_clamp_and_snap(t *testing.T) {
	s := ParameterSlider{Min: 0, Max: 10, Step: 2}
	s.SetValue(11)
	if got := s.Value; got != 10 {
		t.Fatalf("clamped value = %v", got)
	}
	s.SetValue(3.1)
	s.Snap()
	if got := s.Value; got != 4 {
		t.Fatalf("snapped value = %v", got)
	}
}

func TestPresetCard_ActivateCommand_returnsPresetCommand(t *testing.T) {
	cmd := PresetCard{ID: voicedsp.PresetID("robot")}.ActivateCommand()
	if got := cmd.(voiceux.SetPresetCommand); got.ID != "robot" {
		t.Fatalf("unexpected command %#v", got)
	}
}
