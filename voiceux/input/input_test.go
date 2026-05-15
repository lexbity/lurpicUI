package input

import (
	"testing"

	"codeburg.org/lexbit/voicedsp"
)

func TestReorder_moves_items(t *testing.T) {
	got := Reorder([]int{1, 2, 3, 4}, 1, 3)
	want := []int{1, 3, 4, 2}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("reorder = %#v, want %#v", got, want)
		}
	}
}

func TestDefaultCalibrationSteps_labels(t *testing.T) {
	steps := DefaultCalibrationSteps()
	if len(steps) != 5 {
		t.Fatalf("steps = %d", len(steps))
	}
	if steps[0].Vowel != voicedsp.VowelA || steps[4].Vowel != voicedsp.VowelU {
		t.Fatalf("unexpected calibration sequence %#v", steps)
	}
}
