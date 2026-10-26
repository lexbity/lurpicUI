package runtime

import (
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/facet"
)

func TestInvalidationAfterFrameCaptureSurvivesGenerationSeal(t *testing.T) {
	rt := &Runtime{
		dirtyFacets:      make(map[facet.FacetID]facet.DirtyFlags),
		dirtySources:     make(map[facet.FacetID]string),
		nextDirtyFacets:  make(map[facet.FacetID]facet.DirtyFlags),
		nextDirtySources: make(map[facet.FacetID]string),
	}
	rt.recordInvalidation(11, facet.DirtyLayout, "before-cut")
	if got := rt.copyDirtyFacets()[11]; got != facet.DirtyLayout {
		t.Fatalf("current generation flags = %v, want layout", got)
	}
	rt.beginNextDirtyGeneration()
	rt.recordInvalidation(11, facet.DirtyProjection, "during-frame")
	rt.recordInvalidation(12, facet.DirtyHit, "during-frame")
	rt.sealDirtyGeneration()
	if got := rt.dirtyFacets[11]; got != facet.DirtyProjection {
		t.Fatalf("next frame flags for facet 11 = %v, want projection", got)
	}
	if got := rt.dirtyFacets[12]; got != facet.DirtyHit {
		t.Fatalf("next frame flags for facet 12 = %v, want hit", got)
	}
	if got := rt.dirtySources[11]; got != "during-frame" {
		t.Fatalf("next frame source = %q, want during-frame", got)
	}
}

type requestedTickFacet struct {
	facet.Facet
	tick  facet.TickRole
	calls int
}

func newRequestedTickFacet() *requestedTickFacet {
	f := &requestedTickFacet{Facet: facet.NewFacet()}
	f.tick.OnTick = func(time.Duration) {
		f.calls++
		if f.calls == 1 {
			f.tick.RequestTick()
		}
	}
	f.AddRole(&f.tick)
	f.tick.RequestTick()
	return f
}

func (f *requestedTickFacet) Base() *facet.Facet           { return &f.Facet }
func (f *requestedTickFacet) OnAttach(facet.AttachContext) {}
func (f *requestedTickFacet) OnDetach()                    {}
func (f *requestedTickFacet) OnActivate()                  {}
func (f *requestedTickFacet) OnDeactivate()                {}

func TestRequestedTickWorkRunsOncePerAuthorizedFrame(t *testing.T) {
	root := newRequestedTickFacet()
	rt := mustRuntimeTree(t, root)
	rt.RunOneFrameAt(time.Unix(1, 0))
	if root.calls != 1 || len(rt.activeTickFacets) != 1 {
		t.Fatalf("after first tick: calls=%d active=%d", root.calls, len(rt.activeTickFacets))
	}
	rt.RunOneFrameAt(time.Unix(1, 0).Add(time.Second))
	if root.calls != 2 || len(rt.activeTickFacets) != 0 {
		t.Fatalf("after second tick: calls=%d active=%d", root.calls, len(rt.activeTickFacets))
	}
	rt.RunOneFrameAt(time.Unix(1, 0).Add(2 * time.Second))
	if root.calls != 2 {
		t.Fatalf("unrequested tick ran again: calls=%d", root.calls)
	}
}

func TestFocusPolicyChangesInvalidateOrderWithoutQuietFrameRebuilds(t *testing.T) {
	root := newRuntimeFocusFacet(0)
	child := newRuntimeFocusFacet(1)
	root.Base().AddChildRuntime(child.Base())
	rt := mustRuntimeTree(t, root)
	rt.RunOneFrame()
	if got := rt.focusRebuilds; got != 1 {
		t.Fatalf("initial focus rebuilds = %d, want 1", got)
	}
	rt.RunOneFrame()
	if got := rt.focusRebuilds; got != 1 {
		t.Fatalf("quiet focus rebuilds = %d, want 1", got)
	}
	child.focus.SetTabIndex(4)
	rt.RunOneFrame()
	if got := rt.focusRebuilds; got != 2 {
		t.Fatalf("focus rebuilds after policy update = %d, want 2", got)
	}
	if !rt.focusManager.SetFocus(root) {
		t.Fatal("set root focus")
	}
	rt.focusManager.TabNext()
	if got := rt.focusManager.Focused(); got != child.ID() {
		t.Fatalf("focused after tab = %d, want child %d", got, child.ID())
	}
}
