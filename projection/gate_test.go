package projection

import (
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
)

// TestProjectionGate_emptyBoundsPrunesSubtree proves RX-1 FR-1: a non-layer
// facet arranged to empty bounds emits an empty output, contributes no commands
// and no hit regions, and its subtree is not descended into (a hidden host's
// children must not be projected — the A-1/A-2/A-3 stale-pixel class).
func TestProjectionGate_emptyBoundsPrunesSubtree(t *testing.T) {
	root := newProjectionTestFacet("root", gfx.RectFromXYWH(0, 0, 100, 100))
	child := newProjectionTestFacet("child", gfx.RectFromXYWH(10, 10, 50, 50))
	root.AddChild(&child.Facet)
	attachTree(root)

	// Gate the host exactly the way a Stage hides an inactive exhibit: arranged
	// to zero bounds.
	root.layout.ArrangedBounds = gfx.Rect{}

	sys := NewSystem()
	out := sys.Run(root, FrameInfo{Number: 1, WallTime: time.Unix(0, 0)})

	if sys.EmptyBoundsSkips != 1 {
		t.Fatalf("EmptyBoundsSkips = %d, want 1", sys.EmptyBoundsSkips)
	}
	if root.projectCalls != 0 || child.projectCalls != 0 {
		t.Fatalf("project calls = root:%d child:%d, want 0/0 (gated subtree never projected)", root.projectCalls, child.projectCalls)
	}
	if sys.ProjectedFacets != 0 {
		t.Fatalf("ProjectedFacets = %d, want 0 (gated facets are not projections)", sys.ProjectedFacets)
	}
	if len(out.RenderBatchs) != 0 {
		t.Fatalf("RenderBatchs = %d, want 0", len(out.RenderBatchs))
	}
	if hits := out.HitMap.Entries(); len(hits) != 0 {
		t.Fatalf("hit entries = %d, want 0 (an invisible host contributes no hits)", len(hits))
	}
}

// TestProjectionGate_boundsChangeInvalidatesCache proves FR-1 freshness: a pure
// bounds change (same facet, same content) must not be served from the cache —
// arranged bounds are part of the cache key, and the miss is observable through
// CacheMissesByBounds.
func TestProjectionGate_boundsChangeInvalidatesCache(t *testing.T) {
	root := newProjectionTestFacet("root", gfx.RectFromXYWH(0, 0, 100, 100))
	attachTree(root)

	sys := NewSystem()
	sys.Run(root, FrameInfo{Number: 1, WallTime: time.Unix(0, 0)})
	if sys.CacheMissesByBounds != 0 {
		t.Fatalf("frame 1 CacheMissesByBounds = %d, want 0", sys.CacheMissesByBounds)
	}
	if root.projectCalls != 1 {
		t.Fatalf("frame 1 project calls = %d, want 1", root.projectCalls)
	}

	root.layout.ArrangedBounds = gfx.RectFromXYWH(0, 0, 120, 120)
	sys.Run(root, FrameInfo{Number: 2, WallTime: time.Unix(1, 0)})

	if sys.CacheMissesByBounds != 1 {
		t.Fatalf("frame 2 CacheMissesByBounds = %d, want 1", sys.CacheMissesByBounds)
	}
	if root.projectCalls != 2 {
		t.Fatalf("frame 2 project calls = %d, want 2 (bounds change must re-project)", root.projectCalls)
	}
}

// TestProjectionGate_layerOnZeroArrangedHostProjects proves the RX-1 Q1 layer
// exemption at the subtree-prune step: a gated (empty-arranged) non-layer host
// whose subtree holds a layer facet is descended into, and the layer resolves
// its own bounds via layerCtx and projects — it may paint outside a zero-
// arranged host (the command palette's self-mounted surface is the canonical
// case).
func TestProjectionGate_layerOnZeroArrangedHostProjects(t *testing.T) {
	host := newProjectionTestFacet("host", gfx.Rect{}) // arranged empty, not a layer
	surface := newProjectionTestFacet("surface", gfx.Rect{})
	host.AddChild(&surface.Facet)
	attachTree(host)

	rt := projectionStateRuntimeStub{
		projectionLayerRuntimeStub: projectionLayerRuntimeStub{
			layers: map[facet.FacetID]facet.ProjectionLayer{
				surface.ID(): {
					LayerID:       facet.LayerID(7),
					Bounds:        gfx.RectFromXYWH(10, 10, 80, 40),
					Transform:     gfx.Identity(),
					RecipeVersion: 1,
					ClipPolicy:    facet.ClipNone,
					HitPolicy:     uint8(facet.HitPassThrough),
				},
			},
		},
	}

	sys := NewSystem()
	sys.SetRuntime(rt)
	out := sys.Run(host, FrameInfo{Number: 1, WallTime: time.Unix(0, 0)})

	if sys.EmptyBoundsSkips != 1 {
		t.Fatalf("EmptyBoundsSkips = %d, want 1 (host gated)", sys.EmptyBoundsSkips)
	}
	if host.projectCalls != 0 {
		t.Fatalf("gated host projected %d times, want 0", host.projectCalls)
	}
	if surface.projectCalls != 1 {
		t.Fatalf("layer surface projected %d times, want 1", surface.projectCalls)
	}
	if len(out.RenderBatchs) != 1 {
		t.Fatalf("RenderBatchs = %d, want 1 (the layer)", len(out.RenderBatchs))
	}
}

// TestProjectionGate_clipAncestorResize_reprojectsChild pins the RX-1 §7.6
// self-review edge: clip rects are not themselves in the cache key, but a clip
// derives from ancestor bounds, which ARE in the ancestor's key — so when an
// ancestor resizes, its clip changes, the child's parentChildCtx changes, and
// the child re-projects instead of serving stale clipped output.
func TestProjectionGate_clipAncestorResize_reprojectsChild(t *testing.T) {
	root := newProjectionTestFacet("root", gfx.RectFromXYWH(0, 0, 100, 100))
	root.layout.Parent.Clipping = facet.GroupClipBounds
	child := newProjectionTestFacet("child", gfx.RectFromXYWH(10, 10, 50, 50))
	root.AddChild(&child.Facet)
	attachTree(root)

	sys := NewSystem()
	sys.Run(root, FrameInfo{Number: 1, WallTime: time.Unix(0, 0)})
	if child.projectCalls != 1 {
		t.Fatalf("child projected %d times on frame 1, want 1", child.projectCalls)
	}

	// Resize the clip-ancestor. The child's arranged bounds are unchanged; only
	// the inherited clip (derived from the ancestor's bounds) moves.
	root.layout.ArrangedBounds = gfx.RectFromXYWH(0, 0, 80, 80)
	sys.Run(root, FrameInfo{Number: 2, WallTime: time.Unix(1, 0)})

	if child.projectCalls != 2 {
		t.Fatalf("child projected %d times after ancestor resize, want 2 (clip change must re-project)", child.projectCalls)
	}
}

// TestProjectionGate_quarantinedFacetGatesCleanly proves a poisoned facet
// contributes nothing to the frame while its healthy siblings project normally:
// the gate and the recovery surface compose — a quarantined subtree is skipped,
// and a quarantined leaf still resolves to an empty output.
func TestProjectionGate_quarantinedFacetGatesCleanly(t *testing.T) {
	root := newProjectionTestFacet("root", gfx.RectFromXYWH(0, 0, 100, 100))
	child := newProjectionTestFacet("child", gfx.RectFromXYWH(10, 10, 50, 50))
	root.AddChild(&child.Facet)
	attachTree(root)

	rec := &recoveryStub{}
	rec.poison(child.ID())

	sys := NewSystem()
	sys.SetRuntime(rec)
	out := sys.Run(root, FrameInfo{Number: 1, WallTime: time.Unix(0, 0)})

	if child.projectCalls != 0 {
		t.Fatalf("quarantined child projected %d times, want 0", child.projectCalls)
	}
	if root.projectCalls != 1 {
		t.Fatalf("healthy root projected %d times, want 1", root.projectCalls)
	}
	if len(out.RenderBatchs) != 1 {
		t.Fatalf("RenderBatchs = %d, want 1 (healthy root only)", len(out.RenderBatchs))
	}
}
