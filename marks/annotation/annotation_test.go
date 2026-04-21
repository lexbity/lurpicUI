package annotation

import (
	"reflect"
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/job"
	"codeburg.org/lexbit/lurpicui/layout"
)

func TestLabel_anchor_attached_positioning(t *testing.T) {
	root, host := newAnnotationRootWithHost(t)
	label := &Label{
		ID:        "label",
		Placement: LabelAnchorAttached,
		AnchorRef: &AnchorSourceRef{MarkID: host.AuthoredID(), Anchor: "bounds-center"},
		Offset:    gfx.Point{X: 5, Y: -2},
		Padding:   gfx.Insets{Left: 1, Top: 1, Right: 1, Bottom: 1},
	}
	root.AddChild(label.Base())

	if got, want := label.resolvedPosition(), (gfx.Point{X: 15, Y: 3}); got != want {
		t.Fatalf("resolvedPosition = %#v, want %#v", got, want)
	}
}

func TestBadge_attaches_to_host_anchor(t *testing.T) {
	root, host := newAnnotationRootWithHost(t)
	badge := &Badge{
		ID:     "badge",
		Host:   AnchorSourceRef{MarkID: host.AuthoredID(), Anchor: "bounds-center"},
		Offset: gfx.Point{X: 3, Y: -2},
	}
	root.AddChild(badge.Base())

	if got, want := badge.resolvedPosition(), (gfx.Point{X: 13, Y: 3}); got != want {
		t.Fatalf("resolvedPosition = %#v, want %#v", got, want)
	}
}

func TestCallout_body_placed_relative_to_target(t *testing.T) {
	root, target := newAnnotationRootWithHost(t)
	callout := &Callout{
		ID:        "callout",
		Target:    AnchorSourceRef{MarkID: target.AuthoredID(), Anchor: "bounds-center"},
		Direction: CalloutRight,
		Offset:    gfx.Point{X: 4, Y: 1},
	}
	root.AddChild(callout.Base())

	if got, want := callout.resolvedPosition(), (gfx.Point{X: 14, Y: 6}); got != want {
		t.Fatalf("resolvedPosition = %#v, want %#v", got, want)
	}
	if got := callout.bodyBounds(); got.Min.X < 0 {
		t.Fatalf("bodyBounds should be shifted for CalloutRight, got %#v", got)
	}
}

func TestCallout_leader_line_optional(t *testing.T) {
	root, target := newAnnotationRootWithHost(t)
	callout := &Callout{
		ID:        "callout",
		Target:    AnchorSourceRef{MarkID: target.AuthoredID(), Anchor: "bounds-center"},
		Direction: CalloutBelow,
		WithLine:  true,
	}
	root.AddChild(callout.Base())
	if cmds := callout.project(facet.ProjectionContext{}); !containsCommandType(cmds, gfx.StrokePath{}) {
		t.Fatalf("expected leader line commands, got %#v", cmds)
	}
	callout.WithLine = false
	if cmds := callout.project(facet.ProjectionContext{}); containsCommandType(cmds, gfx.StrokePath{}) {
		t.Fatalf("did not expect leader line commands, got %#v", cmds)
	}
}

func TestConnector_straight_between_anchor_points(t *testing.T) {
	root, host := newAnnotationRootWithHost(t)
	target := &Handle{
		ID:       "target",
		Position: gfx.Point{X: 110, Y: 5},
		Size:     10,
	}
	root.AddChild(target.Base())
	connector := &Connector{
		ID:   "connector",
		Mode: ConnectorStraight,
		From: ConnectorEndpoint{Source: AnchorSourceRef{MarkID: host.AuthoredID(), Anchor: "bounds-center"}},
		To:   ConnectorEndpoint{Source: AnchorSourceRef{MarkID: target.AuthoredID(), Anchor: "bounds-center"}},
	}
	root.AddChild(connector.Base())

	path := connector.routePath()
	if len(path.Segments) < 2 {
		t.Fatalf("path = %#v", path)
	}
	from := path.Segments[0].Pts[0]
	to := path.Segments[len(path.Segments)-1].Pts[0]
	if from != (gfx.Point{X: 10, Y: 5}) || to != (gfx.Point{X: 110, Y: 5}) {
		t.Fatalf("route endpoints = %#v -> %#v", from, to)
	}
}

func TestConnector_quadratic_control_points_deterministic(t *testing.T) {
	root, host := newAnnotationRootWithHost(t)
	target := &Handle{
		ID:       "target",
		Position: gfx.Point{X: 100, Y: 40},
		Size:     10,
	}
	root.AddChild(target.Base())
	connector := &Connector{
		ID:   "connector",
		Mode: ConnectorQuadratic,
		From: ConnectorEndpoint{Source: AnchorSourceRef{MarkID: host.AuthoredID(), Anchor: "bounds-center"}},
		To:   ConnectorEndpoint{Source: AnchorSourceRef{MarkID: target.AuthoredID(), Anchor: "bounds-center"}},
	}
	root.AddChild(connector.Base())

	first := connector.quadraticPath()
	second := connector.quadraticPath()
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("quadratic routes differ: %#v vs %#v", first, second)
	}
}

func TestConnector_cubic_tangent_oriented_arrowhead(t *testing.T) {
	root, host := newAnnotationRootWithHost(t)
	target := &Handle{
		ID:       "target",
		Position: gfx.Point{X: 100, Y: 40},
		Size:     10,
	}
	root.AddChild(target.Base())
	connector := &Connector{
		ID:        "connector",
		Mode:      ConnectorCubic,
		From:      ConnectorEndpoint{Source: AnchorSourceRef{MarkID: host.AuthoredID(), Anchor: "bounds-center"}},
		To:        ConnectorEndpoint{Source: AnchorSourceRef{MarkID: target.AuthoredID(), Anchor: "bounds-center"}},
		ArrowHead: true,
	}
	root.AddChild(connector.Base())

	cmds := connector.project(facet.ProjectionContext{})
	if !containsCommandType(cmds, gfx.FillPath{}) {
		t.Fatal("expected arrowhead fill path")
	}
	path := connector.cubicPath()
	if len(path.Segments) == 0 || path.Segments[len(path.Segments)-1].Verb != gfx.PathCubicTo {
		t.Fatalf("unexpected cubic path %#v", path)
	}
}

func TestConnector_missing_anchor_no_panic(t *testing.T) {
	connector := &Connector{
		ID:   "connector",
		Mode: ConnectorStraight,
		From: ConnectorEndpoint{Source: AnchorSourceRef{MarkID: "missing", Anchor: "bounds-center"}},
		To:   ConnectorEndpoint{Source: AnchorSourceRef{MarkID: "missing", Anchor: "bounds-center"}},
	}
	if got := connector.routePath(); len(got.Segments) != 0 {
		t.Fatalf("expected empty route for missing anchors, got %#v", got)
	}
}

func TestConnector_orthogonal_route_job_discarded_when_stale(t *testing.T) {
	pool := job.NewPool(1)
	defer pool.Shutdown()
	rt := &bufferedRuntime{pool: pool}
	root, host := newAnnotationRootWithHost(t)
	target := &Handle{
		ID:       "target",
		Position: gfx.Point{X: 100, Y: 40},
		Size:     10,
	}
	root.AddChild(target.Base())
	connector := &Connector{
		ID:   "connector",
		Mode: ConnectorOrthogonal,
		From: ConnectorEndpoint{Source: AnchorSourceRef{MarkID: host.AuthoredID(), Anchor: "bounds-center"}},
		To:   ConnectorEndpoint{Source: AnchorSourceRef{MarkID: target.AuthoredID(), Anchor: "bounds-center"}},
	}
	root.AddChild(connector.Base())
	connector.runtime = rt
	connector.requestRoute()
	connector.cacheKey = "sentinel"
	target.Position = gfx.Point{X: 140, Y: 60}
	rt.Flush()
	_ = waitForJobDrain(t, pool)
	if connector.cacheKey != "sentinel" {
		t.Fatalf("expected stale route to be discarded, cacheKey = %q", connector.cacheKey)
	}
	if got := connector.routePath(); len(got.Segments) == 0 {
		t.Fatal("expected synchronous fallback route after stale job discard")
	}
	if connector.cacheKey != connector.routeSignature() {
		t.Fatalf("cacheKey = %q, want current signature %q", connector.cacheKey, connector.routeSignature())
	}
}

func TestConnector_orthogonal_route_cache_reused(t *testing.T) {
	root, host := newAnnotationRootWithHost(t)
	target := &Handle{
		ID:       "target",
		Position: gfx.Point{X: 110, Y: 45},
		Size:     10,
	}
	root.AddChild(target.Base())
	connector := &Connector{
		ID:   "connector",
		Mode: ConnectorOrthogonal,
		From: ConnectorEndpoint{Source: AnchorSourceRef{MarkID: host.AuthoredID(), Anchor: "bounds-center"}},
		To:   ConnectorEndpoint{Source: AnchorSourceRef{MarkID: target.AuthoredID(), Anchor: "bounds-center"}},
	}
	root.AddChild(connector.Base())

	first := connector.routePath()
	second := connector.routePath()
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("routes differ: %#v vs %#v", first, second)
	}
	if connector.cacheHits == 0 {
		t.Fatal("expected cache hit on second routePath call")
	}
}

func TestSymbol_registry_resolves_definition(t *testing.T) {
	name := SymbolName("test-symbol")
	RegisterSymbol(SymbolDefinition{
		Name: name,
		BuildPath: func(size float32) gfx.Path {
			half := size / 2
			return pathFromPoints([]gfx.Point{
				{X: -half, Y: 0},
				{X: 0, Y: half},
				{X: half, Y: 0},
			}, false)
		},
		Anchors: func(size float32) map[string]gfx.Point {
			return map[string]gfx.Point{"east": {X: size / 2, Y: 0}}
		},
	})
	def, ok := ResolveSymbol(name)
	if !ok {
		t.Fatal("expected registered symbol to resolve")
	}
	inst := &SymbolInstance{Symbol: name, Size: 24}
	bounds := inst.localBounds()
	if bounds.Width() <= 0 {
		t.Fatalf("unexpected symbol bounds %#v", bounds)
	}
	anchors := inst.ExportAnchors(layout.AnchorExportContext{})
	if _, ok := anchors["east"]; !ok {
		t.Fatalf("missing propagated anchor: %#v", anchors)
	}
	if def.BuildPath == nil {
		t.Fatal("resolved definition missing BuildPath")
	}
}

func TestIcon_registry_lookup_and_fallback(t *testing.T) {
	name := IconName("test-icon")
	RegisterIcon(IconDefinition{
		Name: name,
		BuildPath: func(size float32) gfx.Path {
			return gfx.RectPath(gfx.RectFromXYWH(-size/2, -size/2, size, size))
		},
	})
	if _, ok := ResolveIcon(name); !ok {
		t.Fatal("expected registered icon to resolve")
	}
	icon := &Icon{Name: "missing", Size: 18}
	if bounds := icon.localBounds(); bounds.IsEmpty() {
		t.Fatal("expected fallback icon bounds")
	}
}

func TestHandle_hit_expansion_larger_than_visual(t *testing.T) {
	handle := &Handle{
		Position:     gfx.Point{X: 0, Y: 0},
		Size:         10,
		HitExpansion: 10,
		Focusable:    true,
		Draggable:    true,
		Shape:        HandleCircle,
	}
	if !handle.Base().HitRole().HitTest(gfx.Point{X: 12, Y: 0}).Hit {
		t.Fatal("expected expanded hit area to include point")
	}
	if handle.Base().HitRole().HitTest(gfx.Point{X: 30, Y: 0}).Hit {
		t.Fatal("expected far point to miss")
	}
	if !handle.Base().FocusRole().Focusable() {
		t.Fatal("expected focus role to reflect focusable flag")
	}
	if handle.Base().InputRole() == nil || !handle.Base().InputRole().OnPointer(facet.PointerEvent{}) {
		t.Fatal("expected draggable handle to expose pointer handling")
	}
}

func TestArea_from_baseline_closes_path(t *testing.T) {
	area := &Area{
		Mode:     AreaFromBaseline,
		PointsA:  []gfx.Point{{X: 0, Y: 10}, {X: 10, Y: 20}, {X: 20, Y: 15}},
		Baseline: 5,
	}
	path := area.localPath()
	if len(path.Segments) == 0 || path.Segments[len(path.Segments)-1].Verb != gfx.PathClose {
		t.Fatalf("expected closed path, got %#v", path)
	}
	anchors := area.ExportAnchors(layout.AnchorExportContext{})
	if _, ok := anchors["bounds-center"]; !ok {
		t.Fatalf("expected bounds-center anchor, got %#v", anchors)
	}
}

func newAnnotationRootWithHost(t *testing.T) (*facet.Facet, *Handle) {
	t.Helper()
	root := facet.NewFacet()
	host := &Handle{
		ID:       "host",
		Position: gfx.Point{X: 10, Y: 5},
		Size:     10,
	}
	root.AddChild(host.Base())
	return &root, host
}

func containsCommandType(cmds *gfx.CommandList, want any) bool {
	if cmds == nil {
		return false
	}
	wantType := reflect.TypeOf(want)
	for _, cmd := range cmds.Commands {
		if reflect.TypeOf(cmd) == wantType {
			return true
		}
	}
	return false
}

type fakeRuntime struct {
	pool *job.Pool
}

func (f *fakeRuntime) Schedule(j job.AnyJob) {
	if f == nil || f.pool == nil || j == nil {
		return
	}
	_ = f.pool.SubmitAny(j, nil)
}

func (f *fakeRuntime) CancelJob(id job.JobID) {}
func (f *fakeRuntime) Invalidate(id facet.FacetID, flags facet.DirtyFlags, source string) {}

type bufferedRuntime struct {
	pool *job.Pool
	jobs []job.AnyJob
}

func (b *bufferedRuntime) Schedule(j job.AnyJob) {
	if b == nil || j == nil {
		return
	}
	b.jobs = append(b.jobs, j)
}

func (b *bufferedRuntime) Flush() {
	if b == nil || b.pool == nil {
		return
	}
	for _, j := range b.jobs {
		_ = b.pool.SubmitAny(j, nil)
	}
	b.jobs = nil
}

func (b *bufferedRuntime) CancelJob(id job.JobID) {}
func (b *bufferedRuntime) Invalidate(id facet.FacetID, flags facet.DirtyFlags, source string) {}

func waitForJobDrain(t *testing.T, pool *job.Pool) bool {
	t.Helper()
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		out := pool.Drain()
		if len(out) > 0 {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("timed out waiting for job drain")
	return false
}
