package runtime

import (
	"image/color"
	"strings"
	"sync/atomic"
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/render"
)

func testLayerRegistry(t *testing.T) *layout.LayerRegistry {
	t.Helper()
	r, err := layout.StandardLayerRegistry()
	if err != nil {
		t.Fatalf("standard layer registry: %v", err)
	}
	return r
}

type backendFixture struct {
	initializeErr   error
	submitErr       error
	submitFailAfter atomic.Int32 // fail Submit after this many calls; 0 = never
	initCount       atomic.Int32
	submitCount     atomic.Int32
	destroyCount    atomic.Int32
	lastFrame       atomic.Pointer[render.Frame]
}

func (s *backendFixture) Initialize(surface render.Surface) error {
	s.initCount.Add(1)
	return s.initializeErr
}
func (s *backendFixture) Submit(frame *render.Frame) error {
	s.submitCount.Add(1)
	s.lastFrame.Store(frame)
	if s.submitFailAfter.Load() > 0 && s.submitCount.Load() >= s.submitFailAfter.Load() && s.submitErr != nil {
		return s.submitErr
	}
	return s.submitErr
}
func (s *backendFixture) Resize(width, height int) error          { return nil }
func (s *backendFixture) Destroy()                                { s.destroyCount.Add(1) }

// recreatableBackend is like backendFixture but also implements
// render.RecreatableBackend for testing the Recreate path.
type recreatableBackend struct {
	backendFixture
	recreateCount atomic.Int32
}

func (b *recreatableBackend) Recreate(surface render.Surface) error {
	b.recreateCount.Add(1)
	return nil
}

var _ render.RecreatableBackend = (*recreatableBackend)(nil)

type recordingBackend struct {
	last            *render.Frame
	submitCount     int
	initializeCount int
	destroyCount    int
	lastSurface     render.Surface
}

func (r *recordingBackend) Initialize(surface render.Surface) error {
	r.initializeCount++
	r.lastSurface = surface
	return nil
}
func (r *recordingBackend) Submit(frame *render.Frame) error {
	r.submitCount++
	r.last = frame
	return nil
}
func (r *recordingBackend) Resize(width, height int) error { return nil }
func (r *recordingBackend) Destroy() {
	r.destroyCount++
}

type runtimeTestFacet struct {
	facet.Facet
	attachCount   int
	activateCount int
	detachOrder   *[]string
	name          string
}

func (f *runtimeTestFacet) Base() *facet.Facet {
	f.Facet.BindImpl(f)
	return &f.Facet
}
func (f *runtimeTestFacet) OnAttach(ctx facet.AttachContext) { f.attachCount++ }
func (f *runtimeTestFacet) OnDetach() {
	if f.detachOrder != nil {
		*f.detachOrder = append(*f.detachOrder, f.name)
	}
}
func (f *runtimeTestFacet) OnActivate()   { f.activateCount++ }
func (f *runtimeTestFacet) OnDeactivate() {}

type runtimeRenderFacet struct {
	facet.Facet
	layout facet.LayoutRole
	render facet.RenderRole
	name   string
}

type runtimeLayerFacet struct {
	facet.Facet
	anchors layout.AnchorSet
}

func (f *runtimeLayerFacet) Base() *facet.Facet {
	f.Facet.BindImpl(f)
	return &f.Facet
}

func (f *runtimeLayerFacet) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	if len(f.anchors) == 0 {
		return nil
	}
	out := make(layout.AnchorSet, len(f.anchors))
	for id, pos := range f.anchors {
		out[id] = pos
	}
	return out
}

type layoutCountLeaf struct {
	facet.Facet
	layout facet.LayoutRole

	measureCount int
	arrangeCount int
	size         gfx.Size
}

func (f *layoutCountLeaf) Base() *facet.Facet {
	f.Facet.BindImpl(f)
	return &f.Facet
}

type runtimeFocusFacet struct {
	facet.Facet
	focus facet.FocusRole
}

func (f *runtimeFocusFacet) Base() *facet.Facet {
	f.Facet.BindImpl(f)
	return &f.Facet
}

func newRuntimeRenderFacet(name string, bounds gfx.Rect, fill color.RGBA) *runtimeRenderFacet {
	f := &runtimeRenderFacet{Facet: facet.NewFacet(), name: name}
	f.layout.OnMeasure = func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
		return facet.MeasureResult{Size: gfx.Size{W: bounds.Width(), H: bounds.Height()}}
	}
	f.layout.OnArrange = func(ctx facet.ArrangeContext, b gfx.Rect) {
		f.layout.ArrangedBounds = b
	}
	f.layout.Child.SupportedPlacement = facet.SupportsGrid | facet.SupportsAnchor | facet.SupportsFree | facet.SupportsLinear
	f.render.OnCollect = func(list *gfx.CommandList, b gfx.Rect) {
		list.Add(gfx.FillRect{Rect: b, Brush: gfx.SolidBrush(gfx.ColorFromRGBA8(fill.R, fill.G, fill.B, fill.A))})
	}
	f.AddRole(&f.layout)
	f.AddRole(&f.render)
	return f
}

func newLayoutCountLeaf(size gfx.Size) *layoutCountLeaf {
	leaf := &layoutCountLeaf{Facet: facet.NewFacet(), size: size}
	leaf.layout.OnMeasure = func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
		leaf.measureCount++
		return facet.MeasureResult{Size: leaf.size}
	}
	leaf.layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		leaf.arrangeCount++
		leaf.layout.ArrangedBounds = bounds
	}
	leaf.AddRole(&leaf.layout)
	return leaf
}

func newRuntimeFocusFacet(tabIndex int) *runtimeFocusFacet {
	f := &runtimeFocusFacet{Facet: facet.NewFacet()}
	f.focus.Focusable = func() bool { return true }
	f.focus.TabIndex = tabIndex
	f.AddRole(&f.focus)
	return f
}

func newRuntimeRenderTree() (*runtimeRenderFacet, *runtimeRenderFacet) {
	root := newRuntimeRenderFacet("root", gfx.RectFromXYWH(0, 0, 200, 200), color.RGBA{R: 10, G: 10, B: 10, A: 255})
	root.layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		root.layout.ArrangedBounds = bounds
		for i, childBase := range root.Base().Children() {
			if childBase == nil {
				continue
			}
			childRole := childBase.LayoutRole()
			if childRole == nil {
				continue
			}
			offset := float32(i * 30)
			childRole.Arrange(facet.ArrangeContext{}, gfx.RectFromXYWH(bounds.Min.X+offset, bounds.Min.Y+offset, 40, 40))
		}
	}
	child := newRuntimeRenderFacet("child", gfx.RectFromXYWH(0, 0, 40, 40), color.RGBA{R: 200, G: 0, B: 0, A: 255})
	return root, child
}

func expectPanicContains(t *testing.T, want string, fn func()) {
	t.Helper()
	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatalf("expected panic containing %q", want)
		}
		msg, _ := recovered.(string)
		if !strings.Contains(msg, want) {
			t.Fatalf("panic %q missing %q", msg, want)
		}
	}()
	fn()
}

func mustRuntime(t *testing.T) *Runtime {
	t.Helper()
	root := facet.NewFacet()
	cfg := DefaultConfig()
	cfg.LayerRegistry = testLayerRegistry(t)
	rt, err := New(cfg, nil, nil, &backendFixture{}, &root)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}
	return rt
}

func mustRuntimeTree(t *testing.T, root facet.FacetImpl) *Runtime {
	t.Helper()
	cfg := DefaultConfig()
	cfg.LayerRegistry = testLayerRegistry(t)
	rt, err := New(cfg, nil, nil, &backendFixture{}, root)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}
	return rt
}

func mustRuntimeWithBackend(t *testing.T, root facet.FacetImpl, backend render.Backend) *Runtime {
	t.Helper()
	cfg := DefaultConfig()
	cfg.LayerRegistry = testLayerRegistry(t)
	rt, err := New(cfg, nil, nil, backend, root)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}
	return rt
}

type testWindow struct {
	width        int
	height       int
	contentScale float32
	imeRect      gfx.Rect
}

func (w *testWindow) Surface() platform.Surface { return nil }
func (w *testWindow) SetTitle(title string)     {}
func (w *testWindow) Size() (width, height int) { return w.width, w.height }
func (w *testWindow) ContentScale() float32 {
	if w != nil && w.contentScale > 0 {
		return w.contentScale
	}
	return 1
}
func (w *testWindow) SetIMECursorRect(rect gfx.Rect) { w.imeRect = rect }
func (w *testWindow) Show()                          {}
func (w *testWindow) Hide()                          {}
func (w *testWindow) Close()                         {}
func (w *testWindow) Destroy()                       {}

var _ platform.App = (*nilApp)(nil)

type nilApp struct{}

func (n *nilApp) Events() platform.EventQueue { return nil }
func (n *nilApp) Destroy()                    {}
