package marks

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/job"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/store"
)

// --- Test marks ---

type baseTestMark struct {
	Core
}

func newBaseTestMark() *baseTestMark {
	m := &baseTestMark{}
	m.Layout.OnMeasure = func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
		return facet.MeasureResult{Size: gfx.Size{W: 100, H: 50}}
	}
	m.Layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		m.Layout.ArrangedBounds = bounds
	}
	m.RegisterRoles()
	return m
}

func (m *baseTestMark) Base() *facet.Facet {
	m.Facet.BindImpl(m)
	return &m.Facet
}

func (m *baseTestMark) OnAttach(ctx facet.AttachContext) { m.Core.OnAttach() }
func (m *baseTestMark) OnDetach()                         { m.Core.OnDetach() }
func (m *baseTestMark) OnActivate()                       { m.Core.OnActivate() }
func (m *baseTestMark) OnDeactivate()                     { m.Core.OnDeactivate() }

type bindingTestMark struct {
	Core
	label Binding[string]
}

func newBindingTestMark(s *store.ValueStore[string]) *bindingTestMark {
	m := &bindingTestMark{}
	m.label = FromStore(s, facet.DirtyProjection)
	m.AddBinding(m.label)
	m.Layout.OnMeasure = func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
		return facet.MeasureResult{Size: gfx.Size{W: 100, H: 50}}
	}
	m.Layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		m.Layout.ArrangedBounds = bounds
	}
	m.RegisterRoles()
	return m
}

func (m *bindingTestMark) Base() *facet.Facet {
	m.Facet.BindImpl(m)
	return &m.Facet
}
func (m *bindingTestMark) OnAttach(ctx facet.AttachContext) { m.Core.OnAttach() }
func (m *bindingTestMark) OnDetach()                         { m.Core.OnDetach() }
func (m *bindingTestMark) OnActivate()                       { m.Core.OnActivate() }
func (m *bindingTestMark) OnDeactivate()                     { m.Core.OnDeactivate() }

type buildCommandsTestMark struct {
	Core
	called bool
}

func newBuildCommandsTestMark() *buildCommandsTestMark {
	m := &buildCommandsTestMark{}
	m.Layout.OnMeasure = func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
		return facet.MeasureResult{Size: gfx.Size{W: 100, H: 50}}
	}
	m.Layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		m.Layout.ArrangedBounds = bounds
	}
	m.BuildCommands = func(ctx facet.ProjectionContext) []gfx.Command {
		m.called = true
		return []gfx.Command{
			gfx.FillRect{
				Rect:  gfx.RectFromXYWH(0, 0, 10, 10),
				Brush: gfx.SolidBrush(gfx.ColorFromRGBA8(255, 0, 0, 255)),
			},
		}
	}
	m.RegisterRoles()
	return m
}

func (m *buildCommandsTestMark) Base() *facet.Facet {
	m.Facet.BindImpl(m)
	return &m.Facet
}
func (m *buildCommandsTestMark) OnAttach(ctx facet.AttachContext) { m.Core.OnAttach() }
func (m *buildCommandsTestMark) OnDetach()                         { m.Core.OnDetach() }
func (m *buildCommandsTestMark) OnActivate()                       { m.Core.OnActivate() }
func (m *buildCommandsTestMark) OnDeactivate()                     { m.Core.OnDeactivate() }

// --- Runtime stub ---

type baseRuntimeStub struct{}

func (baseRuntimeStub) Schedule(j job.AnyJob)                  {}
func (baseRuntimeStub) CancelJob(id job.JobID)                 {}
func (baseRuntimeStub) Invalidate(id facet.FacetID, flags facet.DirtyFlags, source string) {}

// --- Tests ---

func TestBase_register_roles_wires_layout(t *testing.T) {
	m := newBaseTestMark()
	if role := m.LayoutRole(); role == nil {
		t.Fatal("expected LayoutRole to be registered")
	}
}

func TestBase_register_roles_skips_unconfigured_roles(t *testing.T) {
	c := Core{}
	c.RegisterRoles()
	if role := c.LayoutRole(); role != nil {
		t.Fatal("expected LayoutRole to be nil when not configured")
	}
	if role := c.HitRole(); role != nil {
		t.Fatal("expected HitRole to be nil when not configured")
	}
	if role := c.InputRole(); role != nil {
		t.Fatal("expected InputRole to be nil when not configured")
	}
	if role := c.FocusRole(); role != nil {
		t.Fatal("expected FocusRole to be nil when not configured")
	}
	if role := c.ViewportRole(); role != nil {
		t.Fatal("expected ViewportRole to be nil when not configured")
	}
	if role := c.TickRole(); role != nil {
		t.Fatal("expected TickRole to be nil when not configured")
	}
	if role := c.ProjectionRole(); role != nil {
		t.Fatal("expected ProjectionRole to be nil when not configured")
	}
	if role := c.RenderRole(); role != nil {
		t.Fatal("expected RenderRole to be nil when not configured")
	}
}

func TestBase_binding_store_change_invalidates_facet(t *testing.T) {
	s := store.NewValueStore("initial")
	m := newBindingTestMark(s)

	facet.Attach(m, facet.AttachContext{Runtime: baseRuntimeStub{}})

	if flags := m.DirtyFlags(); flags != 0 {
		t.Fatalf("expected clean facet before store change, got %d", flags)
	}

	s.Set("updated")

	flags := m.DirtyFlags()
	if flags&facet.DirtyProjection == 0 {
		t.Fatalf("expected DirtyProjection after store change, got %d", flags)
	}
}

func TestBase_binding_store_change_declared_flags_only(t *testing.T) {
	s := store.NewValueStore("initial")
	m := newBindingTestMark(s)

	facet.Attach(m, facet.AttachContext{Runtime: baseRuntimeStub{}})
	s.Set("updated")

	flags := m.DirtyFlags()
	if flags&facet.DirtyLayout != 0 {
		t.Fatal("expected no DirtyLayout — binding declared only DirtyProjection")
	}
}

func TestBase_binding_unsubscribe_on_detach(t *testing.T) {
	s := store.NewValueStore("initial")
	m := newBindingTestMark(s)

	facet.Attach(m, facet.AttachContext{Runtime: baseRuntimeStub{}})
	s.Set("updated")

	if flags := m.DirtyFlags(); flags&facet.DirtyProjection == 0 {
		t.Fatal("expected dirty flags after store change while attached")
	}

	m.ClearDirty(facet.DirtyAll)
	facet.Dispose(m)

	s.Set("again")

	if flags := m.DirtyFlags(); flags != 0 {
		t.Fatal("expected no dirty flags after detach — subscription was cleaned up")
	}
}

func TestBase_default_anchors_returns_five_corners(t *testing.T) {
	c := Core{}
	bounds := gfx.RectFromXYWH(10, 20, 100, 50)

	anchors := c.DefaultAnchors(bounds, layout.AnchorExportContext{})
	if len(anchors) != 5 {
		t.Fatalf("expected 5 default anchors, got %d", len(anchors))
	}

	cases := []struct {
		name string
		want gfx.Point
	}{
		{"bounds_center", gfx.Point{60, 45}},
		{"bounds_top_left", gfx.Point{10, 20}},
		{"bounds_top_right", gfx.Point{110, 20}},
		{"bounds_bottom_left", gfx.Point{10, 70}},
		{"bounds_bottom_right", gfx.Point{110, 70}},
	}
	for _, c := range cases {
		got, ok := anchors[layout.AnchorID(c.name)]
		if !ok {
			t.Fatalf("missing anchor %q", c.name)
		}
		if got != c.want {
			t.Errorf("anchor %q = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestBase_default_anchors_empty_bounds_returns_nil(t *testing.T) {
	c := Core{}
	anchors := c.DefaultAnchors(gfx.Rect{}, layout.AnchorExportContext{})
	if anchors != nil {
		t.Fatal("expected nil anchors for empty bounds")
	}
}

func TestBase_default_anchors_falls_back_to_layer_bounds(t *testing.T) {
	c := Core{}
	layerBounds := gfx.RectFromXYWH(5, 5, 80, 30)

	anchors := c.DefaultAnchors(gfx.Rect{}, layout.AnchorExportContext{
		ResolvedLayer: layout.ResolvedLayer{Bounds: layerBounds},
	})
	if anchors == nil {
		t.Fatal("expected anchors from layer bounds fallback")
	}
	if got := anchors["bounds_center"]; got != (gfx.Point{X: 45, Y: 20}) {
		t.Fatalf("bounds_center from layer = %v, want %v", got, gfx.Point{X: 45, Y: 20})
	}
}

func TestBase_build_commands_wires_projection(t *testing.T) {
	m := newBuildCommandsTestMark()
	if role := m.ProjectionRole(); role == nil {
		t.Fatal("expected ProjectionRole to be registered via BuildCommands")
	}

	ctx := facet.ProjectionContext{
		Bounds:       gfx.RectFromXYWH(0, 0, 100, 50),
		ContentScale: 1,
	}
	list := m.Projection.Project(ctx)
	if list == nil {
		t.Fatal("expected non-nil CommandList from BuildCommands")
	}
	if len(list.Commands) != 1 {
		t.Fatalf("expected 1 command, got %d", len(list.Commands))
	}
	if _, ok := list.Commands[0].(gfx.FillRect); !ok {
		t.Fatalf("expected FillRect command, got %T", list.Commands[0])
	}
	if !m.called {
		t.Fatal("expected BuildCommands to be called")
	}
}

func TestBase_build_commands_empty_returns_nil(t *testing.T) {
	m := &buildCommandsTestMark{}
	m.Layout.OnMeasure = func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
		return facet.MeasureResult{Size: gfx.Size{W: 100, H: 50}}
	}
	m.Layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		m.Layout.ArrangedBounds = bounds
	}
	m.BuildCommands = func(ctx facet.ProjectionContext) []gfx.Command {
		return nil
	}
	m.RegisterRoles()

	ctx := facet.ProjectionContext{
		Bounds:       gfx.RectFromXYWH(0, 0, 100, 50),
		ContentScale: 1,
	}
	list := m.Projection.Project(ctx)
	if list != nil {
		t.Fatal("expected nil CommandList when BuildCommands returns nil")
	}
}

func TestBase_add_binding_skips_const(t *testing.T) {
	c := Core{}
	c.AddBinding(Const("hello"))
	if len(c.subscriptions) != 0 {
		t.Fatal("expected const binding to not be added to subscriptions")
	}
}

func TestBase_add_binding_skips_nil(t *testing.T) {
	c := Core{}
	c.AddBinding(nil)
	if len(c.subscriptions) != 0 {
		t.Fatal("expected nil binding to not be added")
	}
}

type multiBindMark struct {
	Core
	b1 Binding[string]
	b2 Binding[int]
}

func (m *multiBindMark) Base() *facet.Facet {
	m.Facet.BindImpl(m)
	return &m.Facet
}
func (m *multiBindMark) OnAttach(ctx facet.AttachContext) { m.Core.OnAttach() }
func (m *multiBindMark) OnDetach()                         { m.Core.OnDetach() }
func (m *multiBindMark) OnActivate()                       { m.Core.OnActivate() }
func (m *multiBindMark) OnDeactivate()                     { m.Core.OnDeactivate() }

// --- Integration tests ---

type lifecycleTestMark struct {
	Core
	label Binding[string]
	called int
}

func newLifecycleTestMark(s *store.ValueStore[string]) *lifecycleTestMark {
	m := &lifecycleTestMark{}
	m.label = FromStore(s, facet.DirtyProjection)
	m.AddBinding(m.label)
	m.Layout.OnMeasure = func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
		return facet.MeasureResult{Size: gfx.Size{W: 100, H: 50}}
	}
	m.Layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		m.Layout.ArrangedBounds = bounds
	}
	m.BuildCommands = func(ctx facet.ProjectionContext) []gfx.Command {
		m.called++
		_ = m.label.Get()
		return []gfx.Command{
			gfx.FillRect{
				Rect:  gfx.RectFromXYWH(0, 0, 10, 10),
				Brush: gfx.SolidBrush(gfx.ColorFromRGBA8(0, 0, 255, 255)),
			},
		}
	}
	m.RegisterRoles()
	return m
}

func (m *lifecycleTestMark) Base() *facet.Facet {
	m.Facet.BindImpl(m)
	return &m.Facet
}
func (m *lifecycleTestMark) OnAttach(ctx facet.AttachContext) { m.Core.OnAttach() }
func (m *lifecycleTestMark) OnDetach()                         { m.Core.OnDetach() }
func (m *lifecycleTestMark) OnActivate()                       { m.Core.OnActivate() }
func (m *lifecycleTestMark) OnDeactivate()                     { m.Core.OnDeactivate() }

func TestBase_full_lifecycle_attach_measure_arrange_project(t *testing.T) {
	s := store.NewValueStore("hello")
	m := newLifecycleTestMark(s)

	facet.Attach(m, facet.AttachContext{Runtime: baseRuntimeStub{}})

	result := m.Layout.Measure(facet.MeasureContext{
		Runtime:      baseRuntimeStub{},
		ContentScale: 1,
	}, facet.Constraints{MaxSize: gfx.Size{W: 500, H: 200}})
	if result.Size.W != 100 || result.Size.H != 50 {
		t.Fatalf("Measure = %#v, want {100,50}", result.Size)
	}

	bounds := gfx.RectFromXYWH(10, 20, 100, 50)
	m.Layout.Arrange(facet.ArrangeContext{}, bounds)
	if m.Layout.ArrangedBounds != bounds {
		t.Fatalf("ArrangedBounds = %#v, want %#v", m.Layout.ArrangedBounds, bounds)
	}

	ctx := facet.ProjectionContext{
		Bounds:       bounds,
		ContentScale: 1,
	}
	list := m.Projection.Project(ctx)
	if list == nil {
		t.Fatal("expected non-nil CommandList")
	}
	if m.called != 1 {
		t.Fatalf("BuildCommands called %d times, want 1", m.called)
	}
}

func TestBase_full_lifecycle_store_change_triggers_reprojection(t *testing.T) {
	s := store.NewValueStore("initial")
	m := newLifecycleTestMark(s)

	facet.Attach(m, facet.AttachContext{Runtime: baseRuntimeStub{}})

	bounds := gfx.RectFromXYWH(0, 0, 100, 50)
	m.Layout.Arrange(facet.ArrangeContext{}, bounds)

	// First projection
	ctx := facet.ProjectionContext{Bounds: bounds, ContentScale: 1}
	m.Projection.Project(ctx)

	s.Set("updated")

	// Dirty flag set by binding subscription
	if flags := m.DirtyFlags(); flags&facet.DirtyProjection == 0 {
		t.Fatal("expected DirtyProjection after store change")
	}

	m.ClearDirty(facet.DirtyProjection)

	// Second projection — BuildCommands reads the new value
	m.Projection.Project(ctx)
	if m.called != 2 {
		t.Fatalf("BuildCommands called %d times, want 2", m.called)
	}
}

type derivedTestMark struct {
	Core
	value Binding[int]
}

func (m *derivedTestMark) Base() *facet.Facet {
	m.Facet.BindImpl(m)
	return &m.Facet
}
func (m *derivedTestMark) OnAttach(ctx facet.AttachContext) { m.Core.OnAttach() }
func (m *derivedTestMark) OnDetach()                         { m.Core.OnDetach() }
func (m *derivedTestMark) OnActivate()                       { m.Core.OnActivate() }
func (m *derivedTestMark) OnDeactivate()                     { m.Core.OnDeactivate() }

func TestBase_binding_with_derived_store(t *testing.T) {
	src := store.NewValueStore(10)
	d := store.NewDerived(func() int { return src.Get() * 2 }, src)

	m := &derivedTestMark{}
	m.value = FromDerived(d, facet.DirtyProjection)
	m.AddBinding(m.value)
	m.Layout.OnMeasure = func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
		return facet.MeasureResult{Size: gfx.Size{W: 100, H: 50}}
	}
	m.Layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		m.Layout.ArrangedBounds = bounds
	}
	m.RegisterRoles()

	facet.Attach(m, facet.AttachContext{Runtime: baseRuntimeStub{}})

	if m.value.Get() != 20 {
		t.Fatalf("initial derived value = %d, want 20", m.value.Get())
	}

	src.Set(20)
	m.value.Get() // triggers Derived recompute

	if m.value.Get() != 40 {
		t.Fatalf("derived value after source change = %d, want 40", m.value.Get())
	}
}

type anchorExportTestMark struct {
	Core
}

func newAnchorExportTestMark() *anchorExportTestMark {
	m := &anchorExportTestMark{}
	m.Layout.OnMeasure = func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
		return facet.MeasureResult{Size: gfx.Size{W: 100, H: 50}}
	}
	m.Layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		m.Layout.ArrangedBounds = bounds
	}
	m.RegisterRoles()
	return m
}

func (m *anchorExportTestMark) Base() *facet.Facet {
	m.Facet.BindImpl(m)
	return &m.Facet
}
func (m *anchorExportTestMark) OnAttach(ctx facet.AttachContext) { m.Core.OnAttach() }
func (m *anchorExportTestMark) OnDetach()                         { m.Core.OnDetach() }
func (m *anchorExportTestMark) OnActivate()                       { m.Core.OnActivate() }
func (m *anchorExportTestMark) OnDeactivate()                     { m.Core.OnDeactivate() }

func (m *anchorExportTestMark) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	return m.DefaultAnchors(m.Layout.ArrangedBounds, ctx)
}

func TestBase_export_anchors_integration(t *testing.T) {
	m := newAnchorExportTestMark()
	bounds := gfx.RectFromXYWH(10, 20, 100, 50)
	m.Layout.ArrangedBounds = bounds

	anchors := m.ExportAnchors(layout.AnchorExportContext{})
	if len(anchors) != 5 {
		t.Fatalf("expected 5 anchors from integration test, got %d", len(anchors))
	}
	if anchors["bounds_center"] != (gfx.Point{60, 45}) {
		t.Fatalf("bounds_center = %v, want %v", anchors["bounds_center"], gfx.Point{60, 45})
	}
}

func TestBase_multiple_bindings_all_invalidate(t *testing.T) {
	s1 := store.NewValueStore("a")
	s2 := store.NewValueStore(0)

	m := &multiBindMark{
		b1: FromStore(s1, facet.DirtyProjection),
		b2: FromStore(s2, facet.DirtyLayout),
	}
	m.AddBinding(m.b1)
	m.AddBinding(m.b2)
	m.Layout.OnMeasure = func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
		return facet.MeasureResult{Size: gfx.Size{W: 100, H: 50}}
	}
	m.Layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		m.Layout.ArrangedBounds = bounds
	}
	m.RegisterRoles()

	facet.Attach(m, facet.AttachContext{Runtime: baseRuntimeStub{}})

	s1.Set("b")
	flags := m.DirtyFlags()
	if flags&facet.DirtyProjection == 0 {
		t.Error("expected DirtyProjection after string binding change")
	}

	m.ClearDirty(facet.DirtyAll)

	s2.Set(42)
	flags = m.DirtyFlags()
	if flags&facet.DirtyLayout == 0 {
		t.Error("expected DirtyLayout after int binding change")
	}
	if flags&facet.DirtyProjection != 0 {
		t.Error("expected no DirtyProjection — int binding declares only DirtyLayout")
	}
}
