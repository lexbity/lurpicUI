package projection

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/voiceux"
	"codeburg.org/lexbit/voicedsp"
)

func TestMeterFromParams_maps_values_and_badges(t *testing.T) {
	snap := MeterFromParams(gfx.RectFromXYWH(0, 0, 100, 200), voiceux.AudioParamsView{
		RMS:       0.5,
		Peak:      0.8,
		Energy:    0.25,
		PitchHz:   220,
		MouthOpen: 0.9,
		VowelConf: 0.7,
		Clipping:  true,
		Dropout:   true,
	})
	if got := len(snap.Layers); got < 8 {
		t.Fatalf("expected meter layers, got %d", got)
	}
	foundClip := false
	for _, layer := range snap.Layers {
		if layer.Name == "clipping_badge" {
			foundClip = true
		}
	}
	if !foundClip {
		t.Fatal("expected clipping badge layer")
	}
}

func TestVowelSpaceFromState_projects_live_point(t *testing.T) {
	snap := VowelSpaceFromState(gfx.RectFromXYWH(0, 0, 200, 200), voiceux.AudioParamsView{
		F1Hz:        650,
		F2Hz:        1700,
		Vowel:       voicedsp.VowelA,
		FormantConf: 0.8,
		VowelConf:   0.75,
	}, voiceux.CalibrationStateView{})
	if got := len(snap.Points); got == 0 {
		t.Fatal("expected live point")
	}
	pt := snap.Points[0].Point
	if pt.X < 0 || pt.X > 200 || pt.Y < 0 || pt.Y > 200 {
		t.Fatalf("projected point out of bounds: %#v", pt)
	}
}

func TestMapFormants_is_deterministic(t *testing.T) {
	a := MapFormants(gfx.RectFromXYWH(10, 10, 100, 100), 400, 1200)
	b := MapFormants(gfx.RectFromXYWH(10, 10, 100, 100), 400, 1200)
	if a != b {
		t.Fatalf("expected deterministic mapping, got %#v and %#v", a, b)
	}
}
