package runtime

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
)

// projectingMark projects a fill command from its OnProject. It is used to
// verify the phase guard: projecting through the runtime outside the frame's
// projection phase must panic.
type projectingMark struct {
	facet.Facet
}

func newProjectingMark() *projectingMark {
	m := &projectingMark{Facet: facet.NewFacet()}
	proj := &facet.ProjectionRole{}
	proj.OnProject = func(ctx facet.ProjectionContext) *gfx.CommandList {
		var list gfx.CommandList
		list.Add(gfx.FillRect{Rect: ctx.Bounds, Brush: gfx.SolidBrush(gfx.ColorFromRGBA8(1, 2, 3, 255))})
		return &list
	}
	render := &facet.RenderRole{}
	render.OnCollect = func(list *gfx.CommandList, bounds gfx.Rect) {
		list.Add(gfx.FillRect{Rect: bounds, Brush: gfx.SolidBrush(gfx.ColorFromRGBA8(1, 2, 3, 255))})
	}
	layout := &facet.LayoutRole{}
	layout.OnMeasure = func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
		return facet.MeasureResult{Size: c.Constrain(c.MaxSize)}
	}
	layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		layout.ArrangedBounds = bounds
	}
	m.AddRole(proj)
	m.AddRole(render)
	m.AddRole(layout)
	return m
}

// TestProjectionGuardProjectOutsidePhasePanics pins the RX-1 P5 guard: invoking
// ProjectionRole.Project through a live runtime outside the collect→project→
// compose phase must panic.
func TestProjectionGuardProjectOutsidePhasePanics(t *testing.T) {
	mark := newProjectingMark()
	rt := mustRuntimeTree(t, mark)
	rt.RunOneFrame()

	ctx := facet.ProjectionContext{
		Bounds:       mark.Base().LayoutRole().ArrangedBounds,
		Runtime:      rt,
		ContentScale: 1,
	}
	expectPanicContains(t, "outside the projection phase", func() {
		mark.Base().ProjectionRole().Project(ctx)
	})
}

// TestProjectionGuardProjectInsidePhaseSucceeds pins the opposite half: the
// same invocation is legal while the runtime's projection phase is executing
// (as the projection walk itself does).
func TestProjectionGuardProjectInsidePhaseSucceeds(t *testing.T) {
	mark := newProjectingMark()
	rt := mustRuntimeTree(t, mark)

	// Simulate the phase: the runtime's projection-in-progress flag is on while
	// the walk runs, exactly as Run does.
	rt.projectionInProgress.Store(true)
	ctx := facet.ProjectionContext{
		Bounds:       mark.Base().LayoutRole().ArrangedBounds,
		Runtime:      rt,
		ContentScale: 1,
	}
	cmds := mark.Base().ProjectionRole().Project(ctx)
	rt.projectionInProgress.Store(false)
	if cmds == nil || len(cmds.Commands) == 0 {
		t.Fatal("in-phase projection produced no commands")
	}
}

// TestProjectionGuardCollectOutsidePhasePanics pins the Collect guard: a live
// runtime's RenderRole.Collect outside the phase panics.
func TestProjectionGuardCollectOutsidePhasePanics(t *testing.T) {
	mark := newProjectingMark()
	rt := mustRuntimeTree(t, mark)
	rt.RunOneFrame()

	expectPanicContains(t, "outside the projection phase", func() {
		mark.Base().RenderRole().Collect(mark.Base().LayoutRole().ArrangedBounds)
	})
}

// TestProjectionGuardStubRuntimeUnguarded pins the isolation escape hatch: a
// mark projected against a stub runtime (no phase assertion) is not under the
// frame pipeline and must not panic — the button golden and viz-probe tests
// depend on this.
func TestProjectionGuardStubRuntimeUnguarded(t *testing.T) {
	mark := newProjectingMark()
	ctx := facet.ProjectionContext{
		Bounds:       gfx.RectFromXYWH(0, 0, 40, 20),
		ContentScale: 1,
	}
	cmds := mark.Base().ProjectionRole().Project(ctx)
	if cmds == nil || len(cmds.Commands) == 0 {
		t.Fatal("stub-runtime projection produced no commands")
	}
}
