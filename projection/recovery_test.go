package projection

import (
	"sync"
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
)

// recoveryStub is a facetRecovery test double: it recovers projection-callback
// panics and quarantines the facet, mirroring the runtime's guardedInvoke and
// poison contract so projection's use of the facetRecovery surface can be
// tested without importing runtime (which would be an import cycle). The read
// path uses an RWMutex read lock to mirror the runtime's poisonMu cost model.
type recoveryStub struct {
	projectionStateRuntimeStub
	mu       sync.RWMutex
	poisoned map[facet.FacetID]struct{}
}

func (r *recoveryStub) GuardedProject(id facet.FacetID, role string, fn func()) bool {
	if r.isPoisoned(id) {
		return false
	}
	defer func() {
		if p := recover(); p != nil {
			r.poison(id)
		}
	}()
	fn()
	return !r.isPoisoned(id)
}

func (r *recoveryStub) IsPoisoned(id facet.FacetID) bool { return r.isPoisoned(id) }

func (r *recoveryStub) poison(id facet.FacetID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.poisoned == nil {
		r.poisoned = make(map[facet.FacetID]struct{})
	}
	r.poisoned[id] = struct{}{}
}

func (r *recoveryStub) isPoisoned(id facet.FacetID) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.poisoned == nil {
		return false
	}
	_, ok := r.poisoned[id]
	return ok
}

// newPanicProjectFacet is a projection facet whose OnProject panics.
func newPanicProjectFacet(name string, bounds gfx.Rect) *projectionTestFacet {
	f := newProjectionTestFacet(name, bounds)
	f.projection.OnProject = func(ctx facet.ProjectionContext) *gfx.CommandList {
		f.projectCalls++
		panic("boom")
	}
	return f
}

// newPanicCollectFacet is a render-only facet whose OnCollect panics.
func newPanicCollectFacet(name string, bounds gfx.Rect) *projectionTestFacet {
	f := newRenderOnlyFacet(name, bounds)
	f.render.OnCollect = func(list *gfx.CommandList, b gfx.Rect) {
		f.collectCalls++
		panic("boom")
	}
	return f
}

func outputFor(id facet.FacetID, outputs []*ProjectionOutput) *ProjectionOutput {
	for _, po := range outputs {
		if po != nil && po.FacetID == id {
			return po
		}
	}
	return nil
}

func TestProject_PanicInOnProject_Quarantined(t *testing.T) {
	root := newProjectionTestFacet("root", gfx.RectFromXYWH(0, 0, 200, 200))
	panicChild := newPanicProjectFacet("panic", gfx.RectFromXYWH(0, 0, 20, 20))
	sibling := newProjectionTestFacet("sibling", gfx.RectFromXYWH(30, 0, 20, 20))
	root.AddChild(&panicChild.Facet)
	root.AddChild(&sibling.Facet)
	attachTree(root)

	sys := NewSystem()
	rec := &recoveryStub{}
	sys.SetRuntime(rec)

	if out := sys.Run(root, FrameInfo{}); out == nil {
		t.Fatal("expected frame output")
	}
	if !rec.isPoisoned(panicChild.Base().ID()) {
		t.Fatal("expected the panicking facet to be quarantined")
	}
	if po := outputFor(panicChild.Base().ID(), sys.frameOutputs); po == nil || po.Commands.Len() != 0 {
		t.Fatal("panicking facet produced non-empty commands")
	}
	if root.projectCalls != 1 {
		t.Fatalf("root project calls = %d, want 1", root.projectCalls)
	}
	if sibling.projectCalls != 1 {
		t.Fatalf("sibling project calls = %d, want 1", sibling.projectCalls)
	}
}

func TestProject_PanicInOnCollect_Quarantined(t *testing.T) {
	root := newProjectionTestFacet("root", gfx.RectFromXYWH(0, 0, 200, 200))
	panicChild := newPanicCollectFacet("panic", gfx.RectFromXYWH(0, 0, 20, 20))
	sibling := newRenderOnlyFacet("sibling", gfx.RectFromXYWH(30, 0, 20, 20))
	root.AddChild(&panicChild.Facet)
	root.AddChild(&sibling.Facet)
	attachTree(root)

	sys := NewSystem()
	rec := &recoveryStub{}
	sys.SetRuntime(rec)

	if out := sys.Run(root, FrameInfo{}); out == nil {
		t.Fatal("expected frame output")
	}
	if !rec.isPoisoned(panicChild.Base().ID()) {
		t.Fatal("expected the panicking collect facet to be quarantined")
	}
	if po := outputFor(panicChild.Base().ID(), sys.frameOutputs); po == nil || po.Commands.Len() != 0 {
		t.Fatal("panicking collect facet produced non-empty commands")
	}
	if sibling.collectCalls != 1 {
		t.Fatalf("sibling collect calls = %d, want 1", sibling.collectCalls)
	}
}

func TestProject_PoisonedFacet_SkippedInSubsequentRun(t *testing.T) {
	root := newProjectionTestFacet("root", gfx.RectFromXYWH(0, 0, 100, 100))
	panicChild := newPanicProjectFacet("panic", gfx.RectFromXYWH(0, 0, 20, 20))
	root.AddChild(&panicChild.Facet)
	attachTree(root)

	sys := NewSystem()
	rec := &recoveryStub{}
	sys.SetRuntime(rec)

	sys.Run(root, FrameInfo{})
	if panicChild.projectCalls != 1 {
		t.Fatalf("project calls after frame 1 = %d, want 1", panicChild.projectCalls)
	}
	if !rec.isPoisoned(panicChild.Base().ID()) {
		t.Fatal("expected facet to be quarantined after frame 1")
	}

	// Force a re-projection on the next frame; the quarantined facet's
	// callback must not be invoked again and the walk prunes its subtree.
	sys.MarkDirty(panicChild.Base().ID())
	sys.Run(root, FrameInfo{})
	if panicChild.projectCalls != 1 {
		t.Fatalf("project calls after frame 2 = %d, want 1 (poisoned facet skipped)", panicChild.projectCalls)
	}
	if po := outputFor(panicChild.Base().ID(), sys.frameOutputs); po != nil {
		t.Fatalf("poisoned facet produced an output on frame 2 (commands=%d); it must be pruned", po.Commands.Len())
	}
}

func TestProject_PoisonedSubtree_NotWalked(t *testing.T) {
	root := newProjectionTestFacet("root", gfx.RectFromXYWH(0, 0, 100, 100))
	parent := newProjectionTestFacet("parent", gfx.RectFromXYWH(0, 0, 50, 50))
	grandchild := newProjectionTestFacet("grandchild", gfx.RectFromXYWH(0, 0, 20, 20))
	parent.AddChild(&grandchild.Facet)
	root.AddChild(&parent.Facet)
	attachTree(root)

	// Pre-poison the parent and its subtree: projection must not descend into
	// it, and the poisoned facets' callbacks must not run.
	rec := &recoveryStub{poisoned: map[facet.FacetID]struct{}{
		parent.Base().ID():     {},
		grandchild.Base().ID(): {},
	}}
	sys := NewSystem()
	sys.SetRuntime(rec)

	sys.Run(root, FrameInfo{})
	if root.projectCalls != 1 {
		t.Fatalf("root project calls = %d, want 1 (root is not poisoned)", root.projectCalls)
	}
	if parent.projectCalls != 0 {
		t.Fatalf("parent project calls = %d, want 0 (poisoned)", parent.projectCalls)
	}
	if grandchild.projectCalls != 0 {
		t.Fatalf("grandchild project calls = %d, want 0 (poisoned subtree must not be walked)", grandchild.projectCalls)
	}
}

func TestProject_QuarantinesPanickingForkedSubtree(t *testing.T) {
	root := newProjectionTestFacet("root", gfx.RectFromXYWH(0, 0, 480, 32))
	const panicIdx = 4
	children := make([]*projectionTestFacet, 0, 9)
	for i := 0; i < 9; i++ {
		var child *projectionTestFacet
		if i == panicIdx {
			child = newPanicProjectFacet("panic", gfx.RectFromXYWH(float32(i*16), 0, 12, 12))
		} else {
			child = newProjectionTestFacet("child", gfx.RectFromXYWH(float32(i*16), 0, 12, 12))
		}
		root.AddChild(&child.Facet)
		children = append(children, child)
	}
	attachTree(root)

	sys := NewSystem()
	rec := &recoveryStub{}
	sys.SetRuntime(rec)

	// A panic in a forked worker must be recovered (not a process crash) and
	// quarantined; every sibling still projects.
	if out := sys.Run(root, FrameInfo{}); out == nil {
		t.Fatal("expected frame output")
	}
	if !rec.isPoisoned(children[panicIdx].Base().ID()) {
		t.Fatal("expected the panicking forked facet to be quarantined")
	}
	for i, child := range children {
		if i == panicIdx {
			continue
		}
		if child.projectCalls != 1 {
			t.Fatalf("child %d project calls = %d, want 1", i, child.projectCalls)
		}
	}
}

func TestProject_StubRuntime_NoRecovery_NoCrash(t *testing.T) {
	root := newProjectionTestFacet("root", gfx.RectFromXYWH(0, 0, 100, 100))
	panicChild := newPanicProjectFacet("panic", gfx.RectFromXYWH(0, 0, 20, 20))
	root.AddChild(&panicChild.Facet)
	attachTree(root)

	sys := NewSystem()
	// projectionRuntimeStub does NOT implement facetRecovery, so recovery is a
	// no-op and the panic propagates to the caller — preserving the behavior
	// projection's own stub-driven tests rely on.
	sys.SetRuntime(projectionRuntimeStub{})

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected the panic to propagate with a stub runtime")
		}
	}()
	sys.Run(root, FrameInfo{})
}
