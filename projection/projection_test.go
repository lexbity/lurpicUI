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

type projectionLayerRuntimeStub struct {
	projectionRuntimeStub
	layers map[facet.FacetID]facet.ProjectionLayer
}

func (s projectionLayerRuntimeStub) ResolveProjectionLayer(id facet.FacetID) (facet.ProjectionLayer, bool) {
	if s.layers == nil {
		return facet.ProjectionLayer{}, false
	}
	layer, ok := s.layers[id]
	return layer, ok
}

type projectionStateRuntimeStub struct {
	projectionLayerRuntimeStub
	contentScale  float32
	inputModality facet.InputModality
}

func (s projectionStateRuntimeStub) CurrentContentScale() float32 {
	return s.contentScale
}

func (s projectionStateRuntimeStub) CurrentInputModality() facet.InputModality {
	return s.inputModality
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
	f.layout.OnMeasure = func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
		return facet.MeasureResult{Size: gfx.Size{W: bounds.Width(), H: bounds.Height()}}
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
	f.layout.OnMeasure = func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
		return facet.MeasureResult{Size: gfx.Size{W: bounds.Width(), H: bounds.Height()}}
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
	f.layout.OnMeasure = func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
		return facet.MeasureResult{Size: gfx.Size{W: bounds.Width(), H: bounds.Height()}}
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
	if got := len(out.RenderBatchs); got != 2 {
		t.Fatalf("RenderBatchs = %d, want 2", got)
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

func TestProjectionContext_layer_is_populated(t *testing.T) {
	root := newProjectionTestFacet("root", gfx.RectFromXYWH(0, 0, 100, 100))
	child := newProjectionTestFacet("child", gfx.RectFromXYWH(10, 20, 30, 40))
	root.AddChild(&child.Facet)
	attachTree(root)

	sys := NewSystem()
	sys.SetRuntime(projectionLayerRuntimeStub{
		layers: map[facet.FacetID]facet.ProjectionLayer{
			child.ID(): {
				LayerID:       facet.LayerID(22),
				Bounds:        gfx.RectFromXYWH(10, 20, 30, 40),
				Transform:     gfx.Translation(12, 18),
				ClipRect:      gfx.RectFromXYWH(10, 20, 30, 40),
				ClipPolicy:    facet.ClipToParent,
				RecipeVersion: 99,
			},
		},
	})
	sys.Run(root, FrameInfo{})

	if got := child.lastCtx.Layer.Bounds; got != (gfx.RectFromXYWH(10, 20, 30, 40)) {
		t.Fatalf("layer bounds = %#v", got)
	}
	if got := child.lastCtx.Layer.Transform; got != (gfx.Translation(12, 18)) {
		t.Fatalf("layer transform = %#v", got)
	}
	if got := child.lastCtx.Layer.ClipRect; got != (gfx.RectFromXYWH(10, 20, 30, 40)) {
		t.Fatalf("layer clip = %#v", got)
	}
	if got := child.lastCtx.Layer.LayerID; got != facet.LayerID(22) {
		t.Fatalf("layer id = %#v", got)
	}
	if got := child.lastCtx.Layer.ClipPolicy; got != facet.ClipToParent {
		t.Fatalf("layer clip policy = %#v", got)
	}
	if got := child.lastCtx.Layer.RecipeVersion; got != 99 {
		t.Fatalf("layer recipe version = %#v", got)
	}
}

func TestProjectionContext_layer_transform_overrides_viewport(t *testing.T) {
	root := newProjectionTestFacet("root", gfx.RectFromXYWH(0, 0, 100, 100))
	root.viewport.Transform = gfx.Scale(2, 2)
	root.AddRole(&root.viewport)
	child := newProjectionTestFacet("child", gfx.RectFromXYWH(0, 0, 10, 10))
	root.AddChild(&child.Facet)
	attachTree(root)

	sys := NewSystem()
	sys.SetRuntime(projectionLayerRuntimeStub{
		layers: map[facet.FacetID]facet.ProjectionLayer{
			child.ID(): {
				LayerID:   facet.LayerID(22),
				Bounds:    gfx.RectFromXYWH(0, 0, 10, 10),
				Transform: gfx.Identity(),
				ClipRect:  gfx.RectFromXYWH(0, 0, 10, 10),
			},
		},
	})
	out := sys.Run(root, FrameInfo{})

	if child.lastCtx.Layer.Transform != gfx.Identity() {
		t.Fatalf("layer transform = %#v", child.lastCtx.Layer.Transform)
	}
	if got := out.RenderBatchs[1].Transform; got != gfx.Identity() {
		t.Fatalf("batch transform = %#v", got)
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

func TestProjectionSystem_cache_key_includes_layer_and_runtime_state(t *testing.T) {
	root := newProjectionTestFacet("root", gfx.RectFromXYWH(0, 0, 100, 100))
	attachTree(root)

	sys := NewSystem()
	rt := projectionStateRuntimeStub{
		projectionLayerRuntimeStub: projectionLayerRuntimeStub{
			layers: map[facet.FacetID]facet.ProjectionLayer{
				root.ID(): {
					LayerID:       facet.LayerID(11),
					Bounds:        gfx.RectFromXYWH(0, 0, 100, 100),
					Transform:     gfx.Identity(),
					ClipRect:      gfx.RectFromXYWH(0, 0, 100, 100),
					ClipPolicy:    facet.ClipToParent,
					RecipeVersion: 1,
				},
			},
		},
		contentScale:  1.25,
		inputModality: facet.InputModalityPointer,
	}
	sys.SetRuntime(rt)
	sys.Run(root, FrameInfo{})
	first := sys.outputCache[root.ID()].CacheKey
	if got := sys.outputCache[root.ID()].LayerID; got != facet.LayerID(11) {
		t.Fatalf("LayerID = %#v, want 11", got)
	}
	if got := sys.outputCache[root.ID()].InputModality; got != facet.InputModalityPointer {
		t.Fatalf("InputModality = %#v, want pointer", got)
	}
	if got := sys.outputCache[root.ID()].ContentScale; got != 1.25 {
		t.Fatalf("ContentScale = %#v, want 1.25", got)
	}

	rt.layers[root.ID()] = facet.ProjectionLayer{
		LayerID:       facet.LayerID(12),
		Bounds:        gfx.RectFromXYWH(0, 0, 100, 100),
		Transform:     gfx.Identity(),
		ClipRect:      gfx.RectFromXYWH(0, 0, 100, 100),
		ClipPolicy:    facet.ClipToParent,
		RecipeVersion: 1,
	}
	sys.SetRuntime(rt)
	sys.Run(root, FrameInfo{Number: 2})
	if got := sys.outputCache[root.ID()].CacheKey; got == first {
		t.Fatal("expected cache key to change when layer ID changes")
	}
	second := sys.outputCache[root.ID()].CacheKey

	rt.layers[root.ID()] = facet.ProjectionLayer{
		LayerID:       facet.LayerID(12),
		Bounds:        gfx.RectFromXYWH(0, 0, 100, 100),
		Transform:     gfx.Identity(),
		ClipRect:      gfx.RectFromXYWH(0, 0, 100, 100),
		ClipPolicy:    facet.ClipToViewport,
		RecipeVersion: 2,
	}
	rt.contentScale = 2
	rt.inputModality = facet.InputModalityTouch
	sys.SetRuntime(rt)
	sys.Run(root, FrameInfo{Number: 3})
	third := sys.outputCache[root.ID()].CacheKey
	if third == second {
		t.Fatal("expected cache key to change when layer recipe and runtime state change")
	}
}

func TestProjectionSystem_Reset_clears_cached_outputs(t *testing.T) {
	s := NewSystem()
	root := newProjectionTestFacet("root", gfx.RectFromXYWH(0, 0, 10, 10))
	s.Run(root, FrameInfo{})
	if len(s.outputCache) == 0 {
		t.Fatal("expected cache populated after run")
	}
	s.Reset()
	if len(s.outputCache) != 0 {
		t.Fatalf("expected output cache cleared, got %d", len(s.outputCache))
	}
	if len(s.dirtySet) != 0 {
		t.Fatalf("expected dirty set cleared, got %d", len(s.dirtySet))
	}
	if s.currentHitMap != nil {
		t.Fatal("expected hit map cleared")
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

	if got := len(out.RenderBatchs); got != 2 {
		t.Fatalf("RenderBatchs = %d, want 2", got)
	}
	if out.RenderBatchs[0].FacetID != root.ID() || out.RenderBatchs[1].FacetID != child.ID() {
		t.Fatalf("unexpected RenderBatch order: %#v", out.RenderBatchs)
	}
	if out.RenderBatchs[0].Opacity != 1 || out.RenderBatchs[1].Opacity != 1 {
		t.Fatalf("unexpected opacity values: %#v", out.RenderBatchs)
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

func TestHitMap_hitTest_passThroughPolicyFallsThrough(t *testing.T) {
	hm := NewHitMap(
		HitMapEntry{
			FacetID:   2,
			Transform: gfx.Identity(),
			HitPolicy: facet.HitPassThrough,
			Regions: []HitRegion{{
				Bounds: gfx.RectFromXYWH(0, 0, 100, 100),
			}},
		},
		HitMapEntry{
			FacetID:   1,
			Transform: gfx.Identity(),
			Regions: []HitRegion{{
				Bounds: gfx.RectFromXYWH(0, 0, 100, 100),
			}},
		},
	)
	got := hm.HitTest(gfx.Point{X: 10, Y: 10})
	if got == nil || got.FacetID != 1 {
		t.Fatalf("expected pass-through to lower layer, got %#v", got)
	}
}

func TestHitMap_hitTest_blockBelowStopsTraversal(t *testing.T) {
	hm := NewHitMap(
		HitMapEntry{
			FacetID:   2,
			Transform: gfx.Identity(),
			HitPolicy: facet.HitBlockBelow,
			ClipRect:  gfx.RectFromXYWH(0, 0, 5, 5),
			Regions: []HitRegion{{
				Bounds: gfx.RectFromXYWH(0, 0, 100, 100),
			}},
		},
		HitMapEntry{
			FacetID:   1,
			Transform: gfx.Identity(),
			Regions: []HitRegion{{
				Bounds: gfx.RectFromXYWH(0, 0, 100, 100),
			}},
		},
	)
	got := hm.HitTest(gfx.Point{X: 10, Y: 10})
	if got != nil {
		t.Fatalf("expected traversal to stop at blocker, got %#v", got)
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

func TestProjectionSystem_group_clip_wraps_commands_and_child_context(t *testing.T) {
	root := newProjectionTestFacet("root", gfx.RectFromXYWH(0, 0, 100, 100))
	child := newProjectionTestFacet("child", gfx.RectFromXYWH(10, 10, 20, 20))
	root.layout.Parent = facet.GroupParentContract{Clipping: facet.GroupClipBounds}
	root.render.OnCollect = func(list *gfx.CommandList, b gfx.Rect) {
		list.Add(gfx.FillRect{
			Rect:  gfx.RectFromXYWH(0, 0, 220, 220),
			Brush: gfx.SolidBrush(gfx.ColorFromRGBA8(128, 64, 255, 255)),
		})
	}
	root.AddChild(&child.Facet)
	attachTree(root)

	sys := NewSystem()
	out := sys.Run(root, FrameInfo{})

	if out.RenderBatchs == nil || len(out.RenderBatchs) == 0 {
		t.Fatal("expected render batch")
	}
	cmds := out.RenderBatchs[0].Commands.Commands
	if len(cmds) < 3 {
		t.Fatalf("expected clipped command wrapper, got %d commands", len(cmds))
	}
	if _, ok := cmds[0].(gfx.PushClipRect); !ok {
		t.Fatalf("first command = %T, want PushClipRect", cmds[0])
	}
	if _, ok := cmds[len(cmds)-1].(gfx.PopClip); !ok {
		t.Fatalf("last command = %T, want PopClip", cmds[len(cmds)-1])
	}
	ctx := sys.outputCache[root.ID()].ChildContext
	if ctx == nil || ctx.ClipBounds == nil {
		t.Fatal("expected child clip bounds")
	}
	if got := *ctx.ClipBounds; got != (gfx.RectFromXYWH(0, 0, 100, 100)) {
		t.Fatalf("child clip bounds = %#v, want root bounds", got)
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

func TestFrameOutput_RenderBatchs_ordered_back_to_front(t *testing.T) {
	root := newRenderOnlyFacet("root", gfx.RectFromXYWH(0, 0, 100, 100))
	child := newRenderOnlyFacet("child", gfx.RectFromXYWH(10, 10, 20, 20))
	root.AddChild(&child.Facet)
	attachTree(root)

	sys := NewSystem()
	out := sys.Run(root, FrameInfo{})

	if len(out.RenderBatchs) != 2 {
		t.Fatalf("RenderBatchs = %d, want 2", len(out.RenderBatchs))
	}
	if out.RenderBatchs[0].FacetID != root.ID() || out.RenderBatchs[1].FacetID != child.ID() {
		t.Fatalf("unexpected RenderBatch order: %#v", out.RenderBatchs)
	}
}

func TestFrameOutput_empty_commandlist_excluded(t *testing.T) {
	root := newHitOnlyFacet("root", gfx.RectFromXYWH(0, 0, 100, 100))
	attachTree(root)

	sys := NewSystem()
	out := sys.Run(root, FrameInfo{})

	if len(out.RenderBatchs) != 0 {
		t.Fatalf("RenderBatchs = %d, want 0", len(out.RenderBatchs))
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
