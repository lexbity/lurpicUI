package diagnostics

import (
	"strings"
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/job"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/projection"
	"codeburg.org/lexbit/lurpicui/render"
	"codeburg.org/lexbit/lurpicui/theme"
)

type diagTestFacet struct {
	facet.Facet
	name string
}

func newDiagTestFacet(name string) *diagTestFacet {
	return &diagTestFacet{Facet: facet.NewFacet(), name: name}
}

func (f *diagTestFacet) Base() *facet.Facet {
	f.BindImpl(f)
	return &f.Facet
}
func (f *diagTestFacet) OnAttach(ctx facet.AttachContext) {}
func (f *diagTestFacet) OnDetach()                        {}
func (f *diagTestFacet) OnActivate()                      {}
func (f *diagTestFacet) OnDeactivate()                    {}

type diagRuntimeStub struct{}

func (diagRuntimeStub) Schedule(j job.AnyJob)  {}
func (diagRuntimeStub) CancelJob(id job.JobID) {}
func (diagRuntimeStub) Invalidate(id facet.FacetID, flags facet.DirtyFlags, source string) {
}

func buildDiagTree(t *testing.T) (*diagTestFacet, *diagTestFacet, *diagTestFacet) {
	t.Helper()
	root := newDiagTestFacet("root")
	child := newDiagTestFacet("child")
	grand := newDiagTestFacet("grand")
	root.AddChild(&child.Facet)
	child.AddChild(&grand.Facet)
	root.AddRole(&facet.LayoutRole{
		ArrangedBounds: gfx.RectFromXYWH(0, 0, 100, 100),
		OnMeasure: func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
			return facet.MeasureResult{Size: gfx.Size{W: 100, H: 100}}
		},
	})
	child.AddRole(&facet.RenderRole{})
	grand.AddRole(&facet.HitRole{OnHitTest: func(p gfx.Point) facet.HitResult {
		return facet.HitResult{Hit: true, MarkID: 7, Cursor: facet.CursorPointer}
	}})
	facet.Attach(root, facet.AttachContext{Runtime: diagRuntimeStub{}, Theme: theme.Default()})
	facet.Activate(root)
	return root, child, grand
}

type fakeLayerSource struct {
	layers map[facet.FacetID][]LayerSnapshot
}

func (s fakeLayerSource) LayerSnapshots(parent facet.FacetID) []LayerSnapshot {
	return s.layers[parent]
}

type fakeAnchorSource struct {
	snaps map[facet.FacetID]AnchorSnapshot
}

func (s fakeAnchorSource) AnchorSnapshot(parent facet.FacetID) (AnchorSnapshot, bool) {
	snap, ok := s.snaps[parent]
	return snap, ok
}

type fakeHitTraceSource struct {
	trace HitTestTrace
}

func (s fakeHitTraceSource) HitTrace() HitTestTrace {
	return s.trace
}

func TestInspector_walk_visits_all_facets(t *testing.T) {
	root, child, grand := buildDiagTree(t)
	inspector := NewInspector(root)
	var ids []facet.FacetID
	inspector.Walk(func(depth int, info FacetInfo) {
		ids = append(ids, info.ID)
	})
	if len(ids) != 3 {
		t.Fatalf("ids = %#v", ids)
	}
	if ids[0] != root.ID() || ids[1] != child.ID() || ids[2] != grand.ID() {
		t.Fatalf("walk order = %#v", ids)
	}
}

func TestInspector_find_by_id(t *testing.T) {
	root, child, _ := buildDiagTree(t)
	info, ok := NewInspector(root).Find(child.ID())
	if !ok {
		t.Fatal("expected facet")
	}
	if info.ID != child.ID() || info.TypeName == "" || info.ChildCount != 1 {
		t.Fatalf("info = %#v", info)
	}
}

func TestInspector_find_unknown_returns_false(t *testing.T) {
	root, _, _ := buildDiagTree(t)
	if _, ok := NewInspector(root).Find(9999); ok {
		t.Fatal("expected not found")
	}
}

func TestInspector_dirtySet_reflects_current(t *testing.T) {
	root, _, _ := buildDiagTree(t)
	root.Base().InvalidateWithSource(facet.DirtyProjection, "testSource")
	dirty := NewInspector(root).DirtySet()
	if flags := dirty[root.ID()]; flags&facet.DirtyProjection == 0 {
		t.Fatalf("dirty set = %#v", dirty)
	}
}

func TestInspector_lastinvalidatedby_populated(t *testing.T) {
	root, _, _ := buildDiagTree(t)
	root.Base().InvalidateWithSource(facet.DirtyProjection, "testSource")
	info, ok := NewInspector(root).Find(root.ID())
	if !ok {
		t.Fatal("expected root")
	}
	if info.LastInvalidatedBy != "testSource" {
		t.Fatalf("last invalidated = %q", info.LastInvalidatedBy)
	}
}

func TestInspector_lastinvalidatedby_cleared_after_projection(t *testing.T) {
	root, _, _ := buildDiagTree(t)
	root.Base().InvalidateWithSource(facet.DirtyLayout, "testSource")
	sys := projection.NewSystem()
	sys.Run(root, projection.FrameInfo{})
	info, ok := NewInspector(root).Find(root.ID())
	if !ok {
		t.Fatal("expected root")
	}
	if info.LastInvalidatedBy != "" {
		t.Fatalf("expected clear invalidation source, got %q", info.LastInvalidatedBy)
	}
}

func TestInspector_describe_includes_layers_and_anchors(t *testing.T) {
	root, child, _ := buildDiagTree(t)
	insp := NewInspector(root)
	insp.SetLayerSource(fakeLayerSource{
		layers: map[facet.FacetID][]LayerSnapshot{
			root.ID(): {{
				LayerID:        7,
				LayerName:      "root-layer",
				WindowBinding:  "primary",
				Placement:      layout.PlacementStack,
				Measurement:    layout.MeasureStructural,
				CoordSpace:     layout.CoordParentLayout,
				RenderOrder:    3,
				HitPolicy:      layout.HitPassThrough,
				FocusTrap:      true,
				FocusRestore:   facet.FocusRestorePrevious,
				RootPolicyKind: "grid",
				RecipeVersion:  11,
				Materialized:   true,
				CommandCount:   2,
				HitRegionCount: 4,
				Bounds:         gfx.RectFromXYWH(1, 2, 3, 4),
				ArrangedChildren: []ArrangedChildSnapshot{{
					FacetID:       child.ID(),
					LayerID:       8,
					WindowBinding: "primary",
					Placement:     facet.PlacementFree,
					HitPolicy:     facet.HitNormal,
					ClipPolicy:    facet.ClipToParent,
					ZPriority:     1,
					Bounds:        gfx.RectFromXYWH(5, 6, 7, 8),
					ClipRect:      gfx.RectFromXYWH(5, 6, 7, 8),
					Materialized:  true,
				}},
			}},
		},
	})
	insp.SetAnchorSource(fakeAnchorSource{
		snaps: map[facet.FacetID]AnchorSnapshot{
			root.ID(): {
				ParentID: root.ID(),
				Version:  9,
				Entries: []AnchorSnapshotEntry{{
					ID:       "mark",
					Position: gfx.Point{X: 11, Y: 12},
					Children: []facet.FacetID{child.ID()},
				}},
			},
		},
	})
	info, ok := insp.Find(root.ID())
	if !ok {
		t.Fatal("expected root")
	}
	if len(info.Layers) != 1 || info.Layers[0].LayerID != 7 {
		t.Fatalf("layers = %#v", info.Layers)
	}
	if info.Layers[0].LayerName != "root-layer" || !info.Layers[0].Materialized || info.Layers[0].CommandCount != 2 || info.Layers[0].HitRegionCount != 4 {
		t.Fatalf("layer metadata = %#v", info.Layers[0])
	}
	if !info.Layers[0].FocusTrap || info.Layers[0].FocusRestore != facet.FocusRestorePrevious {
		t.Fatalf("focus metadata = %#v", info.Layers[0])
	}
	if len(info.Layers[0].ArrangedChildren) != 1 || info.Layers[0].ArrangedChildren[0].FacetID != child.ID() {
		t.Fatalf("arranged children = %#v", info.Layers[0].ArrangedChildren)
	}
	desc := insp.Describe()
	if desc == "" || !strings.Contains(desc, "Layers:") || !strings.Contains(desc, "ArrangedChildren:") || !strings.Contains(desc, "mark") {
		t.Fatalf("describe output = %q", desc)
	}
}

func TestInspector_hitTrace_source(t *testing.T) {
	insp := NewInspector(nil)
	insp.SetHitTraceSource(fakeHitTraceSource{
		trace: HitTestTrace{
			Result: 42,
			TestedLayers: []LayerHitTrace{{
				ParentID:    7,
				LayerID:     3,
				HitPolicy:   layout.HitPassThrough,
				TestedCount: 2,
				HitFacetID:  42,
				StoppedHere: false,
			}},
		},
	})
	got := insp.HitTrace()
	if got.Result != 42 || len(got.TestedLayers) != 1 {
		t.Fatalf("hit trace = %#v", got)
	}
}

func TestHitProbe_at_returns_all_RenderBatchs(t *testing.T) {
	root, child, _ := buildDiagTree(t)
	hitMap := projection.NewHitMap(
		projection.HitMapEntry{
			FacetID:    root.ID(),
			LayerID:    9,
			LayerOrder: 9000,
			Placement:  facet.PlacementFree,
			HitPolicy:  facet.HitPassThrough,
			ClipPolicy: facet.ClipToParent,
			ZPriority:  7,
			Transform:  gfx.Identity(),
			ClipRect:   gfx.RectFromXYWH(0, 0, 50, 50),
			Regions: []projection.HitRegion{{
				Bounds:      gfx.RectFromXYWH(0, 0, 50, 50),
				MarkID:      1,
				Cursor:      facet.CursorPointer,
				PassThrough: true,
			}},
		},
		projection.HitMapEntry{
			FacetID:   child.ID(),
			Transform: gfx.Identity(),
			Regions: []projection.HitRegion{{
				Bounds: gfx.RectFromXYWH(0, 0, 50, 50),
				MarkID: 2,
				Cursor: facet.CursorText,
			}},
		},
	)
	probe := NewHitProbe(root, hitMap)
	got := probe.At(gfx.Point{X: 10, Y: 10})
	if len(got) != 2 {
		t.Fatalf("hits = %#v", got)
	}
	if !got[0].PassThrough || got[0].FacetID != root.ID() || got[1].FacetID != child.ID() {
		t.Fatalf("hits order = %#v", got)
	}
	if got[0].LayerID != 9 || got[0].LayerOrder != 9000 || got[0].Placement != facet.PlacementFree || got[0].HitPolicy != facet.HitPassThrough || got[0].ClipPolicy != facet.ClipToParent || got[0].ZPriority != 7 || got[0].EffectiveClip != (gfx.RectFromXYWH(0, 0, 50, 50)) {
		t.Fatalf("metadata = %#v", got[0])
	}
}

func TestHitProbe_at_respects_clip_and_reports_effective_clip(t *testing.T) {
	root, _, _ := buildDiagTree(t)
	hitMap := projection.NewHitMap(projection.HitMapEntry{
		FacetID:    root.ID(),
		LayerID:    7,
		LayerOrder: 7000,
		Placement:  facet.PlacementFree,
		HitPolicy:  facet.HitNormal,
		ClipPolicy: facet.ClipToParent,
		Transform:  gfx.Identity(),
		ClipRect:   gfx.RectFromXYWH(0, 0, 20, 20),
		Regions: []projection.HitRegion{{
			Bounds: gfx.RectFromXYWH(0, 0, 50, 50),
			MarkID: 1,
		}},
	})
	probe := NewHitProbe(root, hitMap)
	if got := probe.At(gfx.Point{X: 30, Y: 30}); len(got) != 0 {
		t.Fatalf("expected clipped-out point to miss, got %#v", got)
	}
	got := probe.At(gfx.Point{X: 10, Y: 10})
	if len(got) != 1 {
		t.Fatalf("hits = %#v", got)
	}
	if got[0].EffectiveClip != (gfx.RectFromXYWH(0, 0, 20, 20)) {
		t.Fatalf("effective clip = %#v", got[0].EffectiveClip)
	}
}

func TestPanicContext_String_includes_actionable_context(t *testing.T) {
	got := ContractViolationMessage(PanicContext{
		FacetID:    42,
		MarkID:     7,
		LayerID:    9,
		LayerName:  "overlay",
		LayerOrder: 6000,
		Placement:  facet.PlacementFree,
		HitPolicy:  "pass-through",
		ClipPolicy: "clip",
		Guidance:   "use grid placement",
	})
	if got != "layout contract violation: facet 42; mark 7; layer 9 (overlay); order 6000; placement 2; hit policy pass-through; clip policy clip; use grid placement" {
		t.Fatalf("panic context = %q", got)
	}
}

func TestHitProbe_at_empty_region(t *testing.T) {
	probe := NewHitProbe(nil, projection.NewHitMap())
	if got := probe.At(gfx.Point{X: 10, Y: 10}); len(got) != 0 {
		t.Fatalf("hits = %#v", got)
	}
}

func TestFrameLog_record_and_summary(t *testing.T) {
	log := NewFrameLog(10)
	base := time.Unix(0, 0)
	log.mu.Lock()
	log.entries = []FrameLogEntry{
		{
			Stats: FrameStats{
				ProjectedFacets: 1,
				CacheHits:       1,
				JobsCommitted:   2,
				LayoutDuration:  10 * time.Millisecond,
				ProjectDuration: 20 * time.Millisecond,
				RenderDuration:  30 * time.Millisecond,
			},
			Timestamp: base,
		},
		{
			Stats: FrameStats{
				ProjectedFacets: 3,
				CacheHits:       3,
				JobsCommitted:   4,
				LayoutDuration:  15 * time.Millisecond,
				ProjectDuration: 25 * time.Millisecond,
				RenderDuration:  35 * time.Millisecond,
			},
			Timestamp: base.Add(time.Second),
		},
	}
	log.mu.Unlock()

	summary := log.Summary()
	if summary.FrameCount != 2 || summary.AvgProjected != 2 || summary.AvgJobsCommitted != 3 {
		t.Fatalf("summary = %#v", summary)
	}
	if summary.AvgFPS != 1 {
		t.Fatalf("avg fps = %v", summary.AvgFPS)
	}
	if summary.MaxLayoutMs != 15 || summary.MaxProjectMs != 25 || summary.MaxRenderMs != 35 {
		t.Fatalf("summary maxes = %#v", summary)
	}
	if summary.CacheHitRate != 0.5 {
		t.Fatalf("cache hit rate = %v", summary.CacheHitRate)
	}
}

func TestFrameLog_recent_returns_n_entries(t *testing.T) {
	log := NewFrameLog(10)
	for i := 0; i < 10; i++ {
		log.Record(FrameStats{FrameNumber: uint64(i + 1)})
	}
	recent := log.Recent(5)
	if len(recent) != 5 {
		t.Fatalf("recent len = %d", len(recent))
	}
	if recent[0].Stats.FrameNumber != 6 || recent[4].Stats.FrameNumber != 10 {
		t.Fatalf("recent = %#v", recent)
	}
}

func TestFrameLog_rolling_window(t *testing.T) {
	log := NewFrameLog(5)
	for i := 0; i < 7; i++ {
		log.Record(FrameStats{FrameNumber: uint64(i + 1)})
	}
	recent := log.Recent(10)
	if len(recent) != 5 {
		t.Fatalf("recent len = %d", len(recent))
	}
	if recent[0].Stats.FrameNumber != 3 || recent[4].Stats.FrameNumber != 7 {
		t.Fatalf("recent = %#v", recent)
	}
}

// buildOverlayTree creates 3 facets all with LayoutRole so ArrangedBounds is non-zero.
func buildOverlayTree(t *testing.T) (*diagTestFacet, *diagTestFacet, *diagTestFacet) {
	t.Helper()
	makeLayout := func(x, y float32) *facet.LayoutRole {
		return &facet.LayoutRole{
			ArrangedBounds: gfx.RectFromXYWH(x, y, 40, 40),
			OnMeasure: func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
				return facet.MeasureResult{Size: gfx.Size{W: 40, H: 40}}
			},
		}
	}
	root := newDiagTestFacet("root")
	child := newDiagTestFacet("child")
	grand := newDiagTestFacet("grand")
	root.AddChild(&child.Facet)
	child.AddChild(&grand.Facet)
	root.AddRole(makeLayout(0, 0))
	child.AddRole(makeLayout(10, 10))
	grand.AddRole(makeLayout(20, 20))
	facet.Attach(root, facet.AttachContext{Runtime: diagRuntimeStub{}, Theme: theme.Default()})
	facet.Activate(root)
	return root, child, grand
}

func baseFrame() *render.Frame {
	return &render.Frame{
		RenderBatchs: []render.RenderBatch{{Bounds: gfx.RectFromXYWH(0, 0, 200, 100)}},
	}
}

func TestOverlay_inactive_by_default(t *testing.T) {
	if NewOverlay().IsActive() {
		t.Fatal("expected inactive by default")
	}
}

func TestOverlay_toggle_activates(t *testing.T) {
	o := NewOverlay()
	o.Toggle()
	if !o.IsActive() {
		t.Fatal("expected active after first toggle")
	}
}

func TestOverlay_toggle_cycles_modes(t *testing.T) {
	o := NewOverlay()
	for i := 0; i < 4; i++ {
		o.Toggle()
	}
	if o.IsActive() {
		t.Fatal("expected inactive after 4 toggles")
	}
}

func TestOverlay_inject_inactive_noop(t *testing.T) {
	o := NewOverlay()
	frame := baseFrame()
	o.Inject(frame, nil, nil, FrameStats{})
	if len(frame.RenderBatchs) != 1 {
		t.Fatalf("expected 1 RenderBatch, got %d", len(frame.RenderBatchs))
	}
}

func TestOverlay_inject_adds_RenderBatch(t *testing.T) {
	o := NewOverlay()
	o.Toggle()
	frame := baseFrame()
	o.Inject(frame, nil, nil, FrameStats{})
	if len(frame.RenderBatchs) != 2 {
		t.Fatalf("expected 2 RenderBatchs, got %d", len(frame.RenderBatchs))
	}
}

func TestOverlay_inject_RenderBatch_is_topmost(t *testing.T) {
	o := NewOverlay()
	o.Toggle()
	frame := baseFrame()
	o.Inject(frame, nil, nil, FrameStats{})
	last := frame.RenderBatchs[len(frame.RenderBatchs)-1]
	if last.ID != 0 {
		t.Fatalf("overlay RenderBatch ID should be 0 (no cache), got %v", last.ID)
	}
}

func TestOverlay_inject_produces_commands(t *testing.T) {
	o := NewOverlay()
	o.Toggle()
	root, _, _ := buildOverlayTree(t)
	frame := baseFrame()
	o.Inject(frame, NewInspector(root), nil, FrameStats{})
	overlay := frame.RenderBatchs[len(frame.RenderBatchs)-1]
	if overlay.Commands.Len() == 0 {
		t.Fatal("expected non-empty command list from overlay")
	}
}

func TestOverlay_bounds_drawn_for_each_facet(t *testing.T) {
	o := NewOverlay()
	o.Toggle()
	root, _, _ := buildOverlayTree(t)
	frame := baseFrame()
	o.Inject(frame, NewInspector(root), nil, FrameStats{})
	overlay := frame.RenderBatchs[len(frame.RenderBatchs)-1]
	count := 0
	for _, cmd := range overlay.Commands.Commands {
		if _, ok := cmd.(gfx.StrokeRect); ok {
			count++
		}
	}
	if count != 3 {
		t.Fatalf("expected 3 StrokeRect (one per facet), got %d", count)
	}
}

func TestOverlay_dirty_facets_highlighted(t *testing.T) {
	o := NewOverlay()
	o.Toggle()
	o.Toggle() // bounds+dirty mode
	root, _, _ := buildOverlayTree(t)
	root.Base().InvalidateWithSource(facet.DirtyProjection, "test")
	frame := baseFrame()
	o.Inject(frame, NewInspector(root), nil, FrameStats{})
	overlay := frame.RenderBatchs[len(frame.RenderBatchs)-1]
	hasFillRect := false
	for _, cmd := range overlay.Commands.Commands {
		if _, ok := cmd.(gfx.FillRect); ok {
			hasFillRect = true
			break
		}
	}
	if !hasFillRect {
		t.Fatal("expected FillRect for dirty facet highlight")
	}
}

func TestOverlay_timing_bar_present(t *testing.T) {
	o := NewOverlay()
	o.Toggle()
	stats := FrameStats{
		LayoutDuration:  5 * time.Millisecond,
		ProjectDuration: 5 * time.Millisecond,
		RenderDuration:  5 * time.Millisecond,
	}
	frame := baseFrame()
	o.Inject(frame, nil, nil, stats)
	overlay := frame.RenderBatchs[len(frame.RenderBatchs)-1]
	hasFillRect := false
	for _, cmd := range overlay.Commands.Commands {
		if _, ok := cmd.(gfx.FillRect); ok {
			hasFillRect = true
			break
		}
	}
	if !hasFillRect {
		t.Fatal("expected timing bar FillRect in overlay")
	}
}

func TestOverlay_no_cache_id(t *testing.T) {
	o := NewOverlay()
	o.Toggle()
	frame := baseFrame()
	o.Inject(frame, nil, nil, FrameStats{})
	overlay := frame.RenderBatchs[len(frame.RenderBatchs)-1]
	if overlay.ID != 0 {
		t.Fatalf("overlay RenderBatch should have ID=0 (no cache), got %v", overlay.ID)
	}
	for _, cmd := range overlay.Commands.Commands {
		if bl, ok := cmd.(gfx.BeginRenderBatch); ok && bl.CacheID != 0 {
			t.Fatalf("overlay RenderBatch must not contain BeginRenderBatch with non-zero CacheID")
		}
	}
}

func TestOverlay_hit_regions_drawn(t *testing.T) {
	o := NewOverlay()
	for i := 0; i < 3; i++ {
		o.Toggle() // bounds+dirty+hitmap mode
	}
	hitMap := projection.NewHitMap(
		projection.HitMapEntry{
			FacetID:   1,
			Transform: gfx.Identity(),
			Regions: []projection.HitRegion{
				{Bounds: gfx.RectFromXYWH(10, 10, 30, 30), MarkID: 1},
				{Bounds: gfx.RectFromXYWH(50, 50, 20, 20), MarkID: 2},
			},
		},
	)
	probe := NewHitProbe(nil, hitMap)
	frame := baseFrame()
	o.Inject(frame, nil, probe, FrameStats{})
	overlay := frame.RenderBatchs[len(frame.RenderBatchs)-1]
	count := 0
	for _, cmd := range overlay.Commands.Commands {
		if _, ok := cmd.(gfx.StrokeRect); ok {
			count++
		}
	}
	if count != 2 {
		t.Fatalf("expected 2 StrokeRect for 2 hit regions, got %d", count)
	}
}

func TestOverlay_anchor_visualization_draws_connectors(t *testing.T) {
	o := NewOverlay()
	o.Toggle()
	root, child, _ := buildOverlayTree(t)
	insp := NewInspector(root)
	insp.SetAnchorSource(fakeAnchorSource{
		snaps: map[facet.FacetID]AnchorSnapshot{
			root.ID(): {
				ParentID: root.ID(),
				Version:  1,
				Entries: []AnchorSnapshotEntry{{
					ID:       "anchor-a",
					Position: gfx.Point{X: 5, Y: 5},
					Children: []facet.FacetID{child.ID()},
				}},
			},
		},
	})
	frame := baseFrame()
	o.Inject(frame, insp, nil, FrameStats{})
	overlay := frame.RenderBatchs[len(frame.RenderBatchs)-1]
	hasPolyline := false
	hasFillRect := false
	for _, cmd := range overlay.Commands.Commands {
		switch cmd.(type) {
		case gfx.DrawPolyline:
			hasPolyline = true
		case gfx.FillRect:
			hasFillRect = true
		}
	}
	if !hasPolyline || !hasFillRect {
		t.Fatalf("expected anchor crosshair and connector, got %#v", overlay.Commands.Commands)
	}
}
