package space

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
)

func TestTight_returns_equal_min_and_max(t *testing.T) {
	size := gfx.Size{W: 12, H: 34}
	got := Tight(size)
	if got.MinSize != size || got.MaxSize != size {
		t.Fatalf("got %#v", got)
	}
	if !got.IsTight() {
		t.Fatal("expected tight constraints")
	}
}

func TestLoose_sets_only_max(t *testing.T) {
	max := gfx.Size{W: 100, H: 200}
	got := Loose(max)
	if got.MinSize != (gfx.Size{}) || got.MaxSize != max {
		t.Fatalf("got %#v", got)
	}
	if got.IsTight() {
		t.Fatal("expected loose constraints")
	}
}

func TestUnconstrained_returns_zero_values(t *testing.T) {
	got := Unconstrained()
	if got.MinSize != (gfx.Size{}) || got.MaxSize != (gfx.Size{}) {
		t.Fatalf("got %#v", got)
	}
}

func TestConstrain_clamps_to_bounds(t *testing.T) {
	c := Constraints{
		MinSize: gfx.Size{W: 10, H: 20},
		MaxSize: gfx.Size{W: 30, H: 40},
	}
	if got := c.Constrain(gfx.Size{W: 5, H: 50}); got != (gfx.Size{W: 10, H: 40}) {
		t.Fatalf("got %#v", got)
	}
}

func TestConstrain_respects_unbounded_max(t *testing.T) {
	c := Constraints{
		MinSize: gfx.Size{W: 10, H: 20},
	}
	if got := c.Constrain(gfx.Size{W: 5, H: 50}); got != (gfx.Size{W: 10, H: 50}) {
		t.Fatalf("got %#v", got)
	}
}

func TestWithMaxWidth_and_WithMaxHeight_return_copies(t *testing.T) {
	c := Constraints{
		MinSize: gfx.Size{W: 1, H: 2},
		MaxSize: gfx.Size{W: 3, H: 4},
	}
	width := c.WithMaxWidth(10)
	height := c.WithMaxHeight(20)

	if width.MaxSize.W != 10 || width.MaxSize.H != 4 {
		t.Fatalf("width = %#v", width)
	}
	if height.MaxSize.W != 3 || height.MaxSize.H != 20 {
		t.Fatalf("height = %#v", height)
	}
	if c.MaxSize != (gfx.Size{W: 3, H: 4}) {
		t.Fatalf("original mutated: %#v", c)
	}
}
