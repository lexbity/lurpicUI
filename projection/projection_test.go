package projection

import (
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/job"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/text"
)

type projectionRuntimeStub struct{}

func (projectionRuntimeStub) Schedule(j job.AnyJob)  {}
func (projectionRuntimeStub) CancelJob(id job.JobID) {}
func (projectionRuntimeStub) Invalidate(id facet.FacetID, flags facet.DirtyFlags, source string) {
}

type projectionTestFacet struct {
	facet.Facet

	name string

	layout     facet.LayoutRole
	render     facet.RenderRole
	hit        facet.HitRole
	projection facet.ProjectionRole
	viewport   facet.ViewportRole
	textRole   facet.TextRole

	projectCalls int
	collectCalls int
	lastCtx      facet.ProjectionContext
	attachFn     func(facet.AttachContext)
}

func newProjectionTestFacet(name string, bounds gfx.Rect) *projectionTestFacet {
	f := &projectionTestFacet{
		Facet: facet.NewFacet(),
		name:  name,
	}
	f.layout.ArrangedBounds = bounds
	f.layout.OnMeasure = func(c facet.Constraints) gfx.Size {
		return gfx.Size{W: bounds.Width(), H: bounds.Height()}
	}
	f.render.OnCollect = func(list *gfx.CommandList, b gfx.Rect) {
		f.collectCalls++
		list.Add(gfx.FillRect{
			Rect:  b,
			Brush: gfx.SolidBrush(gfx.ColorFromRGBA8(255, 0, 0, 255)),
		})
	}
	f.hit.OnHitTest = func(p gfx.Point) facet.HitResult {
		if bounds.Contains(p) {
			return facet.HitResult{Hit: true, Cursor: facet.CursorPointer}
		}
		return facet.HitResult{}
	}
	f.projection.OnProject = func(ctx facet.ProjectionContext) *gfx.CommandList {
		f.projectCalls++
		f.lastCtx = ctx
		list := &gfx.CommandList{}
		list.Add(gfx.FillRect{
			Rect:  bounds,
			Brush: gfx.SolidBrush(gfx.ColorFromRGBA8(0, 0, 255, 255)),
		})
		return list
	}
	f.AddRole(&f.layout)
	f.AddRole(&f.render)
	f.AddRole(&f.hit)
	f.AddRole(&f.projection)
	return f
}

func newRenderOnlyFacet(name string, bounds gfx.Rect) *projectionTestFacet {
	f := &projectionTestFacet{
		Facet: facet.NewFacet(),
		name:  name,
	}
	f.layout.ArrangedBounds = bounds
	f.layout.OnMeasure = func(c facet.Constraints) gfx.Size {
		return gfx.Size{W: bounds.Width(), H: bounds.Height()}
	}
	f.render.OnCollect = func(list *gfx.CommandList, b gfx.Rect) {
		f.collectCalls++
		list.Add(gfx.FillRect{
			Rect:  b,
			Brush: gfx.SolidBrush(gfx.ColorFromRGBA8(0, 255, 0, 255)),
		})
	}
	f.hit.OnHitTest = func(p gfx.Point) facet.HitResult {
		if bounds.Contains(p) {
			return facet.HitResult{Hit: true, Cursor: facet.CursorPointer}
		}
		return facet.HitResult{}
	}
	f.AddRole(&f.layout)
	f.AddRole(&f.render)
	f.AddRole(&f.hit)
	return f
}

func newHitOnlyFacet(name string, bounds gfx.Rect) *projectionTestFacet {
	f := &projectionTestFacet{
		Facet: facet.NewFacet(),
		name:  name,
	}
	f.layout.ArrangedBounds = bounds
	f.layout.OnMeasure = func(c facet.Constraints) gfx.Size {
		return gfx.Size{W: bounds.Width(), H: bounds.Height()}
	}
	f.hit.OnHitTest = func(p gfx.Point) facet.HitResult {
		if !bounds.Contains(p) {
			return facet.HitResult{}
		}
		return facet.HitResult{Hit: true, Cursor: facet.CursorPointer}
	}
	f.projection.OnProject = func(ctx facet.ProjectionContext) *gfx.CommandList {
		f.projectCalls++
		f.lastCtx = ctx
		return &gfx.CommandList{}
	}
	f.AddRole(&f.layout)
	f.AddRole(&f.hit)
	f.AddRole(&f.projection)
	return f
}

func newTrackedProjectionFacet(name string, bounds gfx.Rect, s *store.ValueStore[int]) *projectionTestFacet {
	f := newProjectionTestFacet(name, bounds)
	f.projection.OnProject = func(ctx facet.ProjectionContext) *gfx.CommandList {
		f.projectCalls++
		f.lastCtx = ctx
		list := &gfx.CommandList{}
		list.Add(gfx.FillRect{
			Rect:  bounds,
			Brush: gfx.SolidBrush(gfx.ColorFromRGBA8(255, 128, 0, 255)),
		})
		return list
	}
	f.attachFn = func(ctx facet.AttachContext) {
		if s != nil {
			facet.Store(facet.Subscribe(f), &s.OnChange, s.Version, func(signal.Change[int]) {})
		}
	}
	return f
}

func newTextProjectionFacet(name string, bounds gfx.Rect) *projectionTestFacet {
	f := newProjectionTestFacet(name, bounds)
	layout := &text.TextLayout{
		Lines: []text.ShapedLine{{
			Runs: []text.GlyphRun{{
				Glyphs:  []text.PositionedGlyph{{GlyphID: 1, Advance: 10, RuneIndex: 0}},
				Bounds:  text.RectFromXYWH(0, 0, 10, 10),
				Advance: 10,
				Text:    "a",
			}},
			Bounds:    text.RectFromXYWH(0, 0, 10, 10),
			FirstRune: 0,
			RuneCount: 1,
		}},
		LineHeight: 10,
		Bounds:     text.RectFromXYWH(0, 0, 10, 10),
	}
	f.textRole.Layout = layout
	f.textRole.Selection = text.TextRange{Start: 0, End: 1}
	f.textRole.CaretPosition = text.TextPosition{Index: 1, Affinity: text.AffinityUpstream}
	f.textRole.CaretVisible = true
	f.AddRole(&f.textRole)
	return f
}

func (f *projectionTestFacet) Base() *facet.Facet {
	f.Facet.BindImpl(f)
	return &f.Facet
}
func (f *projectionTestFacet) OnAttach(ctx facet.AttachContext) {
	if f.attachFn != nil {
		f.attachFn(ctx)
	}
}
func (f *projectionTestFacet) OnDetach()     {}
func (f *projectionTestFacet) OnActivate()   {}
func (f *projectionTestFacet) OnDeactivate() {}

func attachTree(root facet.FacetImpl) {
	facet.Attach(root, facet.AttachContext{})
}

func TestProjectionSystem_initial_run_projects_all(t *testing.T) {
	root := newProjectionTestFacet("root", gfx.RectFromXYWH(0, 0, 100, 100))
	child := newProjectionTestFacet("child", gfx.RectFromXYWH(10, 10, 20, 20))
	root.AddChild(&child.Facet)
	attachTree(root)

	sys := NewSystem()
	out := sys.Run(root, FrameInfo{Number: 1, DeltaTime: time.Millisecond, WallTime: time.Unix(0, 0)})

	if got := sys.ProjectedFacets; got != 2 {
		t.Fatalf("ProjectedFacets = %d, want 2", got)
	}
	if got := sys.CacheHits; got != 0 {
		t.Fatalf("CacheHits = %d, want 0", got)
	}
	if got := len(out.Layers); got != 2 {
		t.Fatalf("layers = %d, want 2", got)
	}
	if root.projectCalls != 1 || child.projectCalls != 1 {
		t.Fatalf("project calls = root:%d child:%d", root.projectCalls, child.projectCalls)
	}
}

func TestProjectionContext_runtime_nil_without_set(t *testing.T) {
	root := newProjectionTestFacet("root", gfx.RectFromXYWH(0, 0, 10, 10))
	attachTree(root)

	sys := NewSystem()
	sys.Run(root, FrameInfo{})

	if root.lastCtx.Runtime != nil {
		t.Fatalf("runtime = %#v", root.lastCtx.Runtime)
	}
}

func TestProjectionContext_runtime_non_nil(t *testing.T) {
	root := newProjectionTestFacet("root", gfx.RectFromXYWH(0, 0, 10, 10))
	attachTree(root)

	sys := NewSystem()
	sys.SetRuntime(projectionRuntimeStub{})
	sys.Run(root, FrameInfo{})

	if root.lastCtx.Runtime == nil {
		t.Fatal("expected runtime")
	}
}

func TestProjectionSystem_clean_facet_uses_cache(t *testing.T) {
	root := newProjectionTestFacet("root", gfx.RectFromXYWH(0, 0, 100, 100))
	child := newProjectionTestFacet("child", gfx.RectFromXYWH(10, 10, 20, 20))
	root.AddChild(&child.Facet)
	attachTree(root)

	sys := NewSystem()
	sys.Run(root, FrameInfo{})
	sys.Run(root, FrameInfo{Number: 2})

	if got := sys.ProjectedFacets; got != 0 {
		t.Fatalf("ProjectedFacets = %d, want 0", got)
	}
	if got := sys.CacheHits; got != 2 {
		t.Fatalf("CacheHits = %d, want 2", got)
	}
	if root.projectCalls != 1 || child.projectCalls != 1 {
		t.Fatalf("project calls = root:%d child:%d", root.projectCalls, child.projectCalls)
	}
}

func TestProjectionSystem_dirty_facet_reprojects(t *testing.T) {
	shared := store.NewValueStore[int](1)
	root := newTrackedProjectionFacet("root", gfx.RectFromXYWH(0, 0, 100, 100), shared)
	child := newProjectionTestFacet("child", gfx.RectFromXYWH(10, 10, 20, 20))
	root.AddChild(&child.Facet)
	attachTree(root)

	sys := NewSystem()
	sys.Run(root, FrameInfo{})
	firstRootKey := sys.outputCache[root.ID()].CacheKey

	shared.Set(2)
	sys.Run(root, FrameInfo{Number: 2})

	if got := sys.ProjectedFacets; got != 1 {
		t.Fatalf("ProjectedFacets = %d, want 1", got)
	}
	if got := sys.CacheHits; got != 1 {
		t.Fatalf("CacheHits = %d, want 1", got)
	}
	if root.projectCalls != 2 {
		t.Fatalf("project calls = %d, want 2", root.projectCalls)
	}
	if got := sys.outputCache[root.ID()].CacheKey; got == firstRootKey {
		t.Fatal("expected root cache key to change after store version bump")
	}
}

func TestProjectionSystem_cache_key_changes_on_bounds_change(t *testing.T) {
	root := newProjectionTestFacet("root", gfx.RectFromXYWH(0, 0, 100, 100))
	attachTree(root)

	sys := NewSystem()
	sys.Run(root, FrameInfo{})
	first := sys.outputCache[root.ID()].CacheKey

	root.layout.ArrangedBounds = gfx.RectFromXYWH(0, 0, 120, 100)
	sys.Run(root, FrameInfo{Number: 2})

	second := sys.outputCache[root.ID()].CacheKey
	if first == second {
		t.Fatal("expected cache key to change when bounds change")
	}
}

func TestProjectionSystem_cache_key_changes_on_transform_change(t *testing.T) {
	root := newProjectionTestFacet("root", gfx.RectFromXYWH(0, 0, 100, 100))
	child := newProjectionTestFacet("child", gfx.RectFromXYWH(10, 10, 20, 20))
	root.viewport.SetPanZoom(gfx.Point{X: 10, Y: 20}, 1)
	root.AddRole(&root.viewport)
	root.AddChild(&child.Facet)
	attachTree(root)

	sys := NewSystem()
	sys.Run(root, FrameInfo{})
	first := sys.outputCache[child.ID()].CacheKey

	root.viewport.SetPanZoom(gfx.Point{X: 25, Y: 40}, 1)
	sys.Run(root, FrameInfo{Number: 2})

	second := sys.outputCache[child.ID()].CacheKey
	if first == second {
		t.Fatal("expected child cache key to change when viewport transform changes")
	}
}

func TestProjectionSystem_cache_key_changes_on_store_version(t *testing.T) {
	s := store.NewValueStore[int](1)
	root := newTrackedProjectionFacet("root", gfx.RectFromXYWH(0, 0, 100, 100), s)
	attachTree(root)

	sys := NewSystem()
	sys.Run(root, FrameInfo{})
	first := sys.outputCache[root.ID()].CacheKey

	s.Set(2)
	sys.Run(root, FrameInfo{Number: 2})

	second := sys.outputCache[root.ID()].CacheKey
	if first == second {
		t.Fatal("expected cache key to change when store version changes")
	}
}

func TestProjectionSystem_tree_order_parent_before_child(t *testing.T) {
	order := make([]facet.FacetID, 0, 2)
	root := newProjectionTestFacet("root", gfx.RectFromXYWH(0, 0, 100, 100))
	child := newProjectionTestFacet("child", gfx.RectFromXYWH(10, 10, 20, 20))
	root.projection.OnProject = func(ctx facet.ProjectionContext) *gfx.CommandList {
		order = append(order, root.ID())
		cl := &gfx.CommandList{}
		cl.Add(gfx.FillRect{Rect: root.layout.ArrangedBounds, Brush: gfx.SolidBrush(gfx.ColorFromRGBA8(0, 0, 255, 255))})
		return cl
	}
	child.projection.OnProject = func(ctx facet.ProjectionContext) *gfx.CommandList {
		order = append(order, child.ID())
		cl := &gfx.CommandList{}
		cl.Add(gfx.FillRect{Rect: child.layout.ArrangedBounds, Brush: gfx.SolidBrush(gfx.ColorFromRGBA8(0, 0, 255, 255))})
		return cl
	}
	root.AddChild(&child.Facet)
	attachTree(root)

	sys := NewSystem()
	sys.Run(root, FrameInfo{})

	if len(order) != 2 || order[0] != root.ID() || order[1] != child.ID() {
		t.Fatalf("unexpected order: %#v", order)
	}
}

func TestProjectionSystem_assembles_frameoutput(t *testing.T) {
	root := newRenderOnlyFacet("root", gfx.RectFromXYWH(0, 0, 100, 100))
	child := newRenderOnlyFacet("child", gfx.RectFromXYWH(10, 10, 20, 20))
	root.AddChild(&child.Facet)
	attachTree(root)

	sys := NewSystem()
	out := sys.Run(root, FrameInfo{})

	if got := len(out.Layers); got != 2 {
		t.Fatalf("layers = %d, want 2", got)
	}
	if out.Layers[0].FacetID != root.ID() || out.Layers[1].FacetID != child.ID() {
		t.Fatalf("unexpected layer order: %#v", out.Layers)
	}
	if out.Layers[0].Opacity != 1 || out.Layers[1].Opacity != 1 {
		t.Fatalf("unexpected opacity values: %#v", out.Layers)
	}
}

func TestHitMap_hitTest_frontmost_wins(t *testing.T) {
	root := newHitOnlyFacet("root", gfx.RectFromXYWH(0, 0, 100, 100))
	child := newHitOnlyFacet("child", gfx.RectFromXYWH(10, 10, 20, 20))
	root.AddChild(&child.Facet)
	attachTree(root)

	sys := NewSystem()
	out := sys.Run(root, FrameInfo{})
	got := out.HitMap.HitTest(gfx.Point{X: 15, Y: 15})

	if got == nil || got.FacetID != child.ID() {
		t.Fatalf("expected child hit, got %#v", got)
	}
}

func TestHitMap_hitTest_transform_applied(t *testing.T) {
	root := newProjectionTestFacet("root", gfx.RectFromXYWH(0, 0, 100, 100))
	child := newHitOnlyFacet("child", gfx.RectFromXYWH(0, 0, 10, 10))
	root.viewport.SetPanZoom(gfx.Point{X: 10, Y: 15}, 1)
	root.AddRole(&root.viewport)
	root.AddChild(&child.Facet)
	attachTree(root)

	sys := NewSystem()
	out := sys.Run(root, FrameInfo{})
	got := out.HitMap.HitTest(gfx.Point{X: 12, Y: 18})

	if got == nil || got.FacetID != child.ID() {
		t.Fatalf("expected transformed hit on child, got %#v", got)
	}
}

func TestHitMap_hitTest_passthroughregion(t *testing.T) {
	hm := buildHitMap([]*ProjectionOutput{
		{
			FacetID:   1,
			Transform: gfx.Identity(),
			HitRegions: []HitRegion{{
				Bounds:      gfx.RectFromXYWH(0, 0, 100, 100),
				PassThrough: false,
				Cursor:      facet.CursorPointer,
			}},
		},
		{
			FacetID:   2,
			Transform: gfx.Identity(),
			HitRegions: []HitRegion{{
				Bounds:      gfx.RectFromXYWH(0, 0, 100, 100),
				PassThrough: true,
				Cursor:      facet.CursorPointer,
			}},
		},
	})
	got := hm.HitTest(gfx.Point{X: 15, Y: 15})

	if got == nil || got.FacetID != 1 {
		t.Fatalf("expected passthrough to root, got %#v", got)
	}
}

func TestHitMap_hitTest_no_hit_returns_nil(t *testing.T) {
	root := newHitOnlyFacet("root", gfx.RectFromXYWH(0, 0, 10, 10))
	attachTree(root)

	sys := NewSystem()
	out := sys.Run(root, FrameInfo{})
	if got := out.HitMap.HitTest(gfx.Point{X: 50, Y: 50}); got != nil {
		t.Fatalf("expected nil hit, got %#v", got)
	}
}

func TestProjectionOutput_selection_geometry_nil_for_non_text(t *testing.T) {
	root := newProjectionTestFacet("root", gfx.RectFromXYWH(0, 0, 100, 100))
	attachTree(root)

	sys := NewSystem()
	out := sys.Run(root, FrameInfo{})

	if out.SelectionGeometries != nil {
		t.Fatalf("expected nil selection geometries, got %#v", out.SelectionGeometries)
	}
	if got := sys.outputCache[root.ID()].SelectionGeometry; got != nil {
		t.Fatalf("expected nil selection geometry on output, got %#v", got)
	}
}

func TestProjectionOutput_selection_geometry_from_text_role(t *testing.T) {
	root := newTextProjectionFacet("root", gfx.RectFromXYWH(0, 0, 100, 100))
	attachTree(root)

	sys := NewSystem()
	out := sys.Run(root, FrameInfo{})

	got := out.SelectionGeometries[root.ID()]
	if got == nil {
		t.Fatal("expected selection geometry for text role")
	}
	if !got.CaretVisible {
		t.Fatal("expected caret visible")
	}
	if got.CaretRect.Width() == 0 {
		t.Fatalf("expected caret rect, got %#v", got.CaretRect)
	}
	if len(got.SelectionRects) != 1 {
		t.Fatalf("expected 1 selection rect, got %d", len(got.SelectionRects))
	}
	if sys.outputCache[root.ID()].SelectionGeometry == nil {
		t.Fatal("expected cached selection geometry")
	}
}

func TestProjectionSystem_child_context_propagated(t *testing.T) {
	root := newProjectionTestFacet("root", gfx.RectFromXYWH(0, 0, 100, 100))
	child := newProjectionTestFacet("child", gfx.RectFromXYWH(10, 10, 20, 20))
	root.viewport.SetPanZoom(gfx.Point{X: 30, Y: 40}, 2)
	root.AddRole(&root.viewport)
	root.AddChild(&child.Facet)
	attachTree(root)

	sys := NewSystem()
	sys.Run(root, FrameInfo{})

	got := sys.outputCache[child.ID()].ChildContext
	if got == nil {
		t.Fatal("expected child context")
	}
	if got.Transform != root.viewport.Transform {
		t.Fatalf("unexpected child transform: %#v", got.Transform)
	}
}

func TestDirtyPropagation_layout_propagates_up(t *testing.T) {
	root := newProjectionTestFacet("root", gfx.RectFromXYWH(0, 0, 100, 100))
	child := newProjectionTestFacet("child", gfx.RectFromXYWH(10, 10, 20, 20))
	root.AddChild(&child.Facet)
	attachTree(root)

	sys := NewSystem()
	sys.Run(root, FrameInfo{})

	dirty := map[facet.FacetID]facet.DirtyFlags{
		child.ID(): facet.DirtyLayout,
	}
	sys.propagateDirty(buildProjectionTree(root), dirty)

	if dirty[root.ID()]&facet.DirtyLayout == 0 {
		t.Fatal("expected layout dirty to propagate to parent")
	}
}

func TestDirtyPropagation_layout_propagates_down(t *testing.T) {
	root := newProjectionTestFacet("root", gfx.RectFromXYWH(0, 0, 100, 100))
	child := newProjectionTestFacet("child", gfx.RectFromXYWH(10, 10, 20, 20))
	sibling := newProjectionTestFacet("sibling", gfx.RectFromXYWH(30, 30, 20, 20))
	root.AddChild(&child.Facet)
	root.AddChild(&sibling.Facet)
	attachTree(root)

	dirty := map[facet.FacetID]facet.DirtyFlags{
		root.ID(): facet.DirtyLayout,
	}
	sys := NewSystem()
	sys.propagateDirty(buildProjectionTree(root), dirty)

	for _, id := range []facet.FacetID{root.ID(), child.ID(), sibling.ID()} {
		flags := dirty[id]
		if flags&facet.DirtyLayout == 0 || flags&facet.DirtyProjection == 0 || flags&facet.DirtyHit == 0 {
			t.Fatalf("expected dirty flags on %d, got %v", id, flags)
		}
	}
}

func TestDirtyPropagation_projection_does_not_propagate_up(t *testing.T) {
	root := newProjectionTestFacet("root", gfx.RectFromXYWH(0, 0, 100, 100))
	child := newProjectionTestFacet("child", gfx.RectFromXYWH(10, 10, 20, 20))
	root.AddChild(&child.Facet)
	attachTree(root)

	dirty := map[facet.FacetID]facet.DirtyFlags{
		child.ID(): facet.DirtyProjection,
	}
	sys := NewSystem()
	sys.propagateDirty(buildProjectionTree(root), dirty)

	if dirty[root.ID()] != 0 {
		t.Fatalf("expected parent to remain clean, got %v", dirty[root.ID()])
	}
}

func TestDirtyPropagation_projection_propagates_down(t *testing.T) {
	root := newProjectionTestFacet("root", gfx.RectFromXYWH(0, 0, 100, 100))
	child := newProjectionTestFacet("child", gfx.RectFromXYWH(10, 10, 20, 20))
	root.AddChild(&child.Facet)
	attachTree(root)

	dirty := map[facet.FacetID]facet.DirtyFlags{
		root.ID(): facet.DirtyProjection,
	}
	sys := NewSystem()
	sys.propagateDirty(buildProjectionTree(root), dirty)

	if dirty[root.ID()]&facet.DirtyProjection == 0 || dirty[root.ID()]&facet.DirtyHit == 0 {
		t.Fatalf("expected root projection dirty to include hit, got %v", dirty[root.ID()])
	}
	if dirty[child.ID()]&facet.DirtyProjection == 0 || dirty[child.ID()]&facet.DirtyHit == 0 {
		t.Fatalf("expected child projection dirty to include hit, got %v", dirty[child.ID()])
	}
}

func TestDirtyPropagation_hit_follows_projection(t *testing.T) {
	root := newProjectionTestFacet("root", gfx.RectFromXYWH(0, 0, 100, 100))
	attachTree(root)

	dirty := map[facet.FacetID]facet.DirtyFlags{
		root.ID(): facet.DirtyProjection,
	}
	sys := NewSystem()
	sys.propagateDirty(buildProjectionTree(root), dirty)

	if dirty[root.ID()]&facet.DirtyHit == 0 {
		t.Fatalf("expected hit dirty to follow projection, got %v", dirty[root.ID()])
	}
}

func TestDirtyPropagation_accumulates_within_frame(t *testing.T) {
	root := newProjectionTestFacet("root", gfx.RectFromXYWH(0, 0, 100, 100))
	a := newProjectionTestFacet("a", gfx.RectFromXYWH(10, 10, 20, 20))
	b := newProjectionTestFacet("b", gfx.RectFromXYWH(30, 30, 20, 20))
	root.AddChild(&a.Facet)
	root.AddChild(&b.Facet)
	attachTree(root)

	sys := NewSystem()
	sys.MarkDirty(root.ID())
	sys.MarkDirty(a.ID())
	sys.MarkDirty(b.ID())
	sys.Run(root, FrameInfo{})

	if got := sys.ProjectedFacets; got != 3 {
		t.Fatalf("ProjectedFacets = %d, want 3", got)
	}
}

func TestFrameOutput_layers_ordered_back_to_front(t *testing.T) {
	root := newRenderOnlyFacet("root", gfx.RectFromXYWH(0, 0, 100, 100))
	child := newRenderOnlyFacet("child", gfx.RectFromXYWH(10, 10, 20, 20))
	root.AddChild(&child.Facet)
	attachTree(root)

	sys := NewSystem()
	out := sys.Run(root, FrameInfo{})

	if len(out.Layers) != 2 {
		t.Fatalf("layers = %d, want 2", len(out.Layers))
	}
	if out.Layers[0].FacetID != root.ID() || out.Layers[1].FacetID != child.ID() {
		t.Fatalf("unexpected layer order: %#v", out.Layers)
	}
}

func TestFrameOutput_empty_commandlist_excluded(t *testing.T) {
	root := newHitOnlyFacet("root", gfx.RectFromXYWH(0, 0, 100, 100))
	attachTree(root)

	sys := NewSystem()
	out := sys.Run(root, FrameInfo{})

	if len(out.Layers) != 0 {
		t.Fatalf("layers = %d, want 0", len(out.Layers))
	}
	if out.HitMap == nil {
		t.Fatal("expected hitmap")
	}
}

func TestFrameOutput_hitmap_includes_all_hit_roles(t *testing.T) {
	root := newHitOnlyFacet("root", gfx.RectFromXYWH(0, 0, 100, 100))
	child := newHitOnlyFacet("child", gfx.RectFromXYWH(10, 10, 20, 20))
	root.AddChild(&child.Facet)
	attachTree(root)

	sys := NewSystem()
	out := sys.Run(root, FrameInfo{})

	if out.HitMap == nil || len(out.HitMap.entries) != 2 {
		t.Fatalf("hitmap entries = %#v", out.HitMap)
	}
}

func TestFrameOutput_hitmap_frontmost_first(t *testing.T) {
	root := newHitOnlyFacet("root", gfx.RectFromXYWH(0, 0, 100, 100))
	child := newHitOnlyFacet("child", gfx.RectFromXYWH(10, 10, 20, 20))
	root.AddChild(&child.Facet)
	attachTree(root)

	sys := NewSystem()
	out := sys.Run(root, FrameInfo{})

	if out.HitMap == nil || len(out.HitMap.entries) != 2 {
		t.Fatalf("hitmap entries = %#v", out.HitMap)
	}
	if out.HitMap.entries[0].facetID != child.ID() || out.HitMap.entries[1].facetID != root.ID() {
		t.Fatalf("unexpected hit order: %#v", out.HitMap.entries)
	}
}
