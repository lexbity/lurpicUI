package input

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
)

func TestCapturesFacetReportsPointerAndTouchOwnership(t *testing.T) {
	const target facet.FacetID = 41
	s := &System{
		pointers: map[PointerID]*pointerState{0: {PressTarget: &captureTarget{FacetID: target}}},
		touches:  map[uint64]*touchState{9: {Target: target, Active: true}},
	}
	if !s.CapturesFacet(target) {
		t.Fatal("active pointer/touch ownership was not reported")
	}
	if s.CapturesFacet(target + 1) {
		t.Fatal("capture observation matched a different facet")
	}
	s.pointers[0].PressTarget = nil
	s.touches[9].Active = false
	if s.CapturesFacet(target) {
		t.Fatal("retired pointer/touch ownership remained visible")
	}
}
