package layout

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
)

func TestAnchorPositionCache_update_get_version(t *testing.T) {
	cache := NewAnchorPositionCache()
	if got := cache.Version(); got != 0 {
		t.Fatalf("unexpected initial version: %d", got)
	}
	if changed := cache.Update("a", gfx.Point{X: 1, Y: 2}); !changed {
		t.Fatal("expected first update to change cache")
	}
	if got := cache.Version(); got != 1 {
		t.Fatalf("unexpected version after first update: %d", got)
	}
	if changed := cache.Update("a", gfx.Point{X: 1, Y: 2}); changed {
		t.Fatal("expected identical update to be ignored")
	}
	if got := cache.Version(); got != 1 {
		t.Fatalf("unexpected version after identical update: %d", got)
	}
	if changed := cache.Update("a", gfx.Point{X: 3, Y: 4}); !changed {
		t.Fatal("expected moved position to change cache")
	}
	if got := cache.Version(); got != 2 {
		t.Fatalf("unexpected version after move: %d", got)
	}
	pos, ok := cache.Get("a")
	if !ok || pos != (gfx.Point{X: 3, Y: 4}) {
		t.Fatalf("unexpected cache value: pos=%#v ok=%v", pos, ok)
	}
	if _, ok := cache.Get("missing"); ok {
		t.Fatal("expected missing anchor to be absent")
	}
}

func TestPlacementHints_zero_value_is_valid(t *testing.T) {
	var hints PlacementHints
	if hints.Align != AlignStretch {
		t.Fatalf("unexpected zero alignment: %v", hints.Align)
	}
	if hints.FreeAnchor != FreeTopLeft {
		t.Fatalf("unexpected zero free anchor: %v", hints.FreeAnchor)
	}
	if hints.AnchorSide != AnchorAbove {
		t.Fatalf("unexpected zero anchor side: %v", hints.AnchorSide)
	}
}
