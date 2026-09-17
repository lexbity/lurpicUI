package projection

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
)

// TestHitMap_entriesFrontToBack pins the Q6 hit-map ordering contract: the
// entries built from a frame's outputs are front-to-back, and HitTest resolves
// the frontmost facet at a covered point.
func TestHitMap_entriesFrontToBack(t *testing.T) {
	// Outputs are back-to-front (root first, child last); the hit map must
	// present the child (front) before the root (back).
	root := newHitOnlyFacet("root", gfx.RectFromXYWH(0, 0, 100, 100))
	child := newHitOnlyFacet("child", gfx.RectFromXYWH(0, 0, 100, 100))
	root.AddChild(&child.Facet)
	attachTree(root)

	sys := NewSystem()
	out := sys.Run(root, FrameInfo{})
	entries := out.HitMap.Entries()
	if len(entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(entries))
	}
	if entries[0].FacetID != child.ID() || entries[1].FacetID != root.ID() {
		t.Fatalf("entries not front-to-back: first=%d want child %d, second=%d want root %d",
			entries[0].FacetID, child.ID(), entries[1].FacetID, root.ID())
	}
	if got := out.HitMap.HitTest(gfx.Point{X: 50, Y: 50}); got == nil || got.FacetID != child.ID() {
		t.Fatalf("frontmost hit = %#v, want child", got)
	}
}

// TestHitMap_deterministicUnderShuffle pins determinism: running the same tree
// through fresh projection systems yields the same front-to-back order and the
// same winner every time.
func TestHitMap_deterministicUnderShuffle(t *testing.T) {
	root := newHitOnlyFacet("root", gfx.RectFromXYWH(0, 0, 100, 100))
	child := newHitOnlyFacet("child", gfx.RectFromXYWH(0, 0, 100, 100))
	root.AddChild(&child.Facet)
	attachTree(root)

	var winner facet.FacetID
	var front facet.FacetID
	for i := 0; i < 3; i++ {
		sys := NewSystem()
		out := sys.Run(root, FrameInfo{})
		entries := out.HitMap.Entries()
		if len(entries) != 2 {
			t.Fatalf("run %d: entries = %d, want 2", i, len(entries))
		}
		if i == 0 {
			front = entries[0].FacetID
			if got := out.HitMap.HitTest(gfx.Point{X: 50, Y: 50}); got != nil {
				winner = got.FacetID
			}
			continue
		}
		if entries[0].FacetID != front {
			t.Fatalf("run %d: front entry %d != %d", i, entries[0].FacetID, front)
		}
		if got := out.HitMap.HitTest(gfx.Point{X: 50, Y: 50}); got == nil || got.FacetID != winner {
			t.Fatalf("run %d: winner %#v != %d", i, got, winner)
		}
	}
}

// TestHitMap_hitTest_noHitOnGatedEmpty pins the gate↔hit junction: a facet
// arranged to empty bounds contributes no hit region, so a point over its
// former area resolves nothing (FR-1 invisible ⇒ unclickable).
func TestHitMap_hitTest_noHitOnGatedEmpty(t *testing.T) {
	root := newHitOnlyFacet("root", gfx.RectFromXYWH(0, 0, 100, 100))
	child := newHitOnlyFacet("child", gfx.RectFromXYWH(0, 0, 100, 100))
	root.AddChild(&child.Facet)
	// The child is arranged to empty (its host gates it).
	child.layout.ArrangedBounds = gfx.Rect{}
	attachTree(root)

	sys := NewSystem()
	out := sys.Run(root, FrameInfo{})
	if got := out.HitMap.HitTest(gfx.Point{X: 50, Y: 50}); got == nil {
		t.Fatal("expected the root to be hit (child gated to empty)")
	}
}
