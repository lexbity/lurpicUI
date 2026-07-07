package studio

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/state"
	"codeburg.org/lexbit/lurpicui/gfx"
)

func TestModeFor_wide(t *testing.T) {
	if got := ModeFor(gfx.Size{W: BreakpointWide, H: 800}); got != state.LayoutWide {
		t.Fatalf("at breakpoint width: expected wide, got %q", got)
	}
	if got := ModeFor(gfx.Size{W: BreakpointWide + 1, H: 800}); got != state.LayoutWide {
		t.Fatalf("above breakpoint: expected wide, got %q", got)
	}
	if got := ModeFor(gfx.Size{W: BreakpointWide + 100, H: 600}); got != state.LayoutWide {
		t.Fatalf("well above breakpoint: expected wide, got %q", got)
	}
}

func TestModeFor_narrow(t *testing.T) {
	if got := ModeFor(gfx.Size{W: BreakpointWide - 1, H: 800}); got != state.LayoutNarrow {
		t.Fatalf("below breakpoint: expected narrow, got %q", got)
	}
	if got := ModeFor(gfx.Size{W: 480, H: 800}); got != state.LayoutNarrow {
		t.Fatalf("phone width: expected narrow, got %q", got)
	}
	if got := ModeFor(gfx.Size{W: 320, H: 480}); got != state.LayoutNarrow {
		t.Fatalf("small phone: expected narrow, got %q", got)
	}
}

func TestModeFor_boundary(t *testing.T) {
	// Exactly at breakpoint -> wide (>=)
	if got := ModeFor(gfx.Size{W: BreakpointWide, H: 600}); got != state.LayoutWide {
		t.Fatalf("exactly at breakpoint: expected wide, got %q", got)
	}
	// One below -> narrow
	if got := ModeFor(gfx.Size{W: BreakpointWide - 1, H: 600}); got != state.LayoutNarrow {
		t.Fatalf("one below breakpoint: expected narrow, got %q", got)
	}
}
