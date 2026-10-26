package projection

import (
	"sync/atomic"
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
)

func TestProjectionCallbacksStaySerialAndPreOrder(t *testing.T) {
	root := newProjectionTestFacet("root", gfx.RectFromXYWH(0, 0, 320, 32))
	var active atomic.Int32
	var maxActive atomic.Int32
	children := make([]*projectionTestFacet, 9)
	for i := range children {
		child := newProjectionTestFacet("child", gfx.RectFromXYWH(float32(i*16), 0, 12, 12))
		childIndex := i
		child.projection.OnProject = func(facet.ProjectionContext) *gfx.CommandList {
			current := active.Add(1)
			for observed := maxActive.Load(); current > observed && !maxActive.CompareAndSwap(observed, current); observed = maxActive.Load() {
			}
			active.Add(-1)
			list := &gfx.CommandList{}
			list.Add(gfx.FillRect{
				Rect:  gfx.RectFromXYWH(float32(childIndex*16), 0, 12, 12),
				Brush: gfx.SolidBrush(gfx.ColorFromRGBA8(0, 0, 255, 255)),
			})
			return list
		}
		root.Base().AddChild(&child.Facet)
		children[i] = child
	}
	attachTree(root)

	sys := NewSystem()
	sys.SetRuntime(projectionStateRuntimeStub{})
	sys.Run(root, FrameInfo{})

	if got := maxActive.Load(); got != 1 {
		t.Fatalf("maximum concurrent projection callbacks = %d, want 1", got)
	}
	if got := len(sys.frameOutputs); got != len(children)+1 {
		t.Fatalf("frame outputs = %d, want %d", got, len(children)+1)
	}
	if sys.frameOutputs[0].FacetID != root.Base().ID() {
		t.Fatalf("first output = %d, want root %d", sys.frameOutputs[0].FacetID, root.Base().ID())
	}
	for i, child := range children {
		if got, want := sys.frameOutputs[i+1].FacetID, child.Base().ID(); got != want {
			t.Fatalf("output %d facet = %d, want %d", i+1, got, want)
		}
	}
}

func TestProjectionTopologyRetainedUntilLifecycleInvalidation(t *testing.T) {
	root := newProjectionTestFacet("root", gfx.RectFromXYWH(0, 0, 80, 24))
	child := newProjectionTestFacet("child", gfx.RectFromXYWH(0, 0, 32, 24))
	root.Base().AddChild(&child.Facet)
	attachTree(root)

	sys := NewSystem()
	sys.SetRuntime(projectionStateRuntimeStub{})
	sys.Run(root, FrameInfo{Number: 1})
	sys.Run(root, FrameInfo{Number: 2})
	if got := sys.TopologyRebuilds; got != 1 {
		t.Fatalf("unchanged topology rebuilds = %d, want 1", got)
	}

	sys.InvalidateTopology()
	sys.MarkDirty(root.Base().ID())
	sys.Run(root, FrameInfo{Number: 3})
	if got := sys.TopologyRebuilds; got != 2 {
		t.Fatalf("topology rebuilds after invalidation = %d, want 2", got)
	}
	if got := len(sys.frameOutputs); got != 2 {
		t.Fatalf("outputs after topology refresh = %d, want 2", got)
	}
}

func TestProjectionTopologyReleaseClearsRetiredReferences(t *testing.T) {
	root := newProjectionTestFacet("root", gfx.RectFromXYWH(0, 0, 80, 24))
	child := newProjectionTestFacet("child", gfx.RectFromXYWH(0, 0, 32, 24))
	root.Base().AddChild(&child.Facet)
	attachTree(root)

	sys := NewSystem()
	sys.SetRuntime(projectionStateRuntimeStub{})
	sys.Run(root, FrameInfo{Number: 1})
	childID := child.Base().ID()
	root.Base().RemoveChild(child.Base())
	sys.ForgetFacet(childID)
	sys.Run(root, FrameInfo{Number: 2})

	if _, ok := sys.outputCache[childID]; ok {
		t.Fatalf("disposed output cache retained facet %d", childID)
	}
	for _, node := range sys.nodeList[:cap(sys.nodeList)] {
		if node != nil && node.base != nil && node.base.ID() == childID {
			t.Fatalf("projection topology backing retained retired facet %d", childID)
		}
	}
	for _, output := range sys.frameOutputs[:cap(sys.frameOutputs)] {
		if output != nil && output.FacetID == childID {
			t.Fatalf("frame output backing retained retired facet %d", childID)
		}
	}
}

func TestRuntimeOwnedProjectionQuietFrameSkipsTreeWalk(t *testing.T) {
	root := newProjectionTestFacet("root", gfx.RectFromXYWH(0, 0, 100, 40))
	child := newProjectionTestFacet("child", gfx.RectFromXYWH(0, 0, 40, 20))
	root.Base().AddChild(&child.Facet)
	attachTree(root)
	sys := NewSystem()
	sys.SetRuntime(projectionStateRuntimeStub{})
	sys.MarkDirtyFlags(root.Base().ID(), facet.DirtyAll)
	sys.MarkDirtyFlags(child.Base().ID(), facet.DirtyAll)
	sys.BeginDirtyGeneration()
	first := sys.Run(root, FrameInfo{Number: 1})
	if first == nil || len(first.RenderBatchs) != 2 {
		t.Fatalf("initial output batches = %d, want 2", len(first.RenderBatchs))
	}
	sys.SealDirtyGeneration()
	rootCalls, childCalls := root.projectCalls, child.projectCalls
	quiet := sys.Run(root, FrameInfo{Number: 2})
	if quiet == nil || len(quiet.RenderBatchs) != 2 {
		t.Fatalf("quiet output batches = %d, want 2", len(quiet.RenderBatchs))
	}
	if sys.GateCount != 0 || sys.ProjectedFacets != 0 || sys.CacheHits != 2 {
		t.Fatalf("quiet projection work gate=%d projected=%d hits=%d, want 0/0/2", sys.GateCount, sys.ProjectedFacets, sys.CacheHits)
	}
	if root.projectCalls != rootCalls || child.projectCalls != childCalls {
		t.Fatalf("quiet callbacks root=%d/%d child=%d/%d", root.projectCalls, rootCalls, child.projectCalls, childCalls)
	}
}

func TestRuntimeOwnedLayoutInvalidationFollowsChangedGeometry(t *testing.T) {
	root := newProjectionTestFacet("root", gfx.RectFromXYWH(0, 0, 100, 40))
	changed := newProjectionTestFacet("changed", gfx.RectFromXYWH(0, 0, 40, 20))
	unchanged := newProjectionTestFacet("unchanged", gfx.RectFromXYWH(50, 0, 40, 20))
	root.Base().AddChild(&changed.Facet)
	root.Base().AddChild(&unchanged.Facet)
	attachTree(root)
	sys := NewSystem()
	sys.SetRuntime(projectionStateRuntimeStub{})
	sys.MarkDirtyFlags(root.Base().ID(), facet.DirtyAll)
	sys.BeginDirtyGeneration()
	sys.Run(root, FrameInfo{Number: 1})
	sys.SealDirtyGeneration()
	rootCalls := root.projectCalls
	changedCalls := changed.projectCalls
	unchangedCalls := unchanged.projectCalls

	changed.layout.ArrangedBounds = gfx.RectFromXYWH(0, 0, 45, 20)
	sys.MarkDirtyFlags(root.Base().ID(), facet.DirtyLayout)
	sys.BeginDirtyGeneration()
	sys.Run(root, FrameInfo{Number: 2})
	sys.SealDirtyGeneration()
	if root.projectCalls != rootCalls+1 {
		t.Fatalf("layout invalidation root callbacks=%d, want %d", root.projectCalls, rootCalls+1)
	}
	if changed.projectCalls != changedCalls+1 {
		t.Fatalf("changed geometry callbacks=%d, want %d", changed.projectCalls, changedCalls+1)
	}
	if unchanged.projectCalls != unchangedCalls {
		t.Fatalf("unchanged sibling callbacks=%d, want %d", unchanged.projectCalls, unchangedCalls)
	}
	if sys.ProjectedFacets != 2 || sys.CacheHits != 1 {
		t.Fatalf("layout projection work projected=%d cacheHits=%d, want 2/1", sys.ProjectedFacets, sys.CacheHits)
	}
}
