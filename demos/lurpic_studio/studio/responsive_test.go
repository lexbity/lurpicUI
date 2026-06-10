package studio

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/state"
	"codeburg.org/lexbit/lurpicui/gfx"
)

func TestModeForWide(t *testing.T) {
	cases := []struct {
		name string
		w    float32
	}{
		{"at breakpoint", breakpointWide},
		{"above breakpoint", breakpointWide + 1},
		{"HD width", 1280},
		{"FHD width", 1920},
		{"4K width", 3840},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mode := ModeFor(gfx.Size{W: tc.w, H: 800})
			if mode != state.LayoutWide {
				t.Errorf("expected LayoutWide for width %f, got %v", tc.w, mode)
			}
		})
	}
}

func TestModeForNarrow(t *testing.T) {
	cases := []struct {
		name string
		w    float32
	}{
		{"just below breakpoint", breakpointWide - 1},
		{"phone portrait", 360},
		{"small phone", 320},
		{"zero width", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mode := ModeFor(gfx.Size{W: tc.w, H: 800})
			if mode != state.LayoutNarrow {
				t.Errorf("expected LayoutNarrow for width %f, got %v", tc.w, mode)
			}
		})
	}
}

func TestModeForBoundary(t *testing.T) {
	wide := ModeFor(gfx.Size{W: breakpointWide, H: 800})
	narrow := ModeFor(gfx.Size{W: breakpointWide - 0.5, H: 800})
	if wide != state.LayoutWide {
		t.Errorf("expected LayoutWide at exactly breakpoint, got %v", wide)
	}
	if narrow != state.LayoutNarrow {
		t.Errorf("expected LayoutNarrow below breakpoint, got %v", narrow)
	}
}

func TestModeForHeightIgnored(t *testing.T) {
	mode := ModeFor(gfx.Size{W: 1280, H: 1})
	if mode != state.LayoutWide {
		t.Errorf("ModeFor should ignore height, got %v", mode)
	}
	mode = ModeFor(gfx.Size{W: 480, H: 2048})
	if mode != state.LayoutNarrow {
		t.Errorf("ModeFor should ignore height, got %v", mode)
	}
}
