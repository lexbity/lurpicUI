package structure

import (
	"strings"
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/projection"
	"codeburg.org/lexbit/lurpicui/render"
	"codeburg.org/lexbit/lurpicui/runtime"
)

type testShapeFacet struct {
	facet.Facet
	id         string
	bounds     gfx.Rect
	viewport   facet.ViewportRole
	layout     facet.LayoutRole
	projection facet.ProjectionRole
	hit        facet.HitRole
}

func newTestShapeFacet(bounds gfx.Rect) *testShapeFacet {
	f := &testShapeFacet{
		Facet:  facet.NewFacet(),
		bounds: bounds,
	}
	f.layout.ArrangedBounds = bounds
	f.layout.OnMeasure = func(c facet.Constraints) gfx.Size {
		return gfx.Size{W: bounds.Width(), H: bounds.Height()}
	}
	f.projection.OnProject = func(ctx facet.ProjectionContext) *gfx.CommandList {
		list := &gfx.CommandList{}
		list.Add(gfx.FillRect{Rect: bounds, Brush: gfx.SolidBrush(gfx.ColorFromRGBA8(255, 0, 0, 255))})
		return list
	}
	f.hit.OnHitTest = func(p gfx.Point) facet.HitResult {
		if bounds.Contains(p) {
			return facet.HitResult{Hit: true, Cursor: facet.CursorPointer}
		}
		return facet.HitResult{}
	}
	f.AddRole(&f.layout)
	f.AddRole(&f.projection)
	f.AddRole(&f.hit)
	return f
}

func (f *testShapeFacet) Base() *facet.Facet {
	f.Facet.BindImpl(f)
	return &f.Facet
}

func (f *testShapeFacet) Descriptor() marks.Descriptor {
	return marks.Descriptor{
		Family:            marks.FamilyStructure,
		ConstructionClass: marks.ConstructionPrimitive,
		Type:              marks.TypeName("structure:test-shape"),
		HitTestable:       true,
	}
}

func (f *testShapeFacet) AuthoredID() string { return f.id }
func (f *testShapeFacet) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	return boundsAnchors(f.bounds)
}

func (f *testShapeFacet) OnAttach(ctx facet.AttachContext) {}
func (f *testShapeFacet) OnDetach()                        {}
func (f *testShapeFacet) OnActivate()                      {}
func (f *testShapeFacet) OnDeactivate()                    {}

func projectStructure(t *testing.T, root facet.FacetImpl) *projection.FrameOutput {
	t.Helper()
	sys := projection.NewSystem()
	attachStructureTree(t, root)
	return sys.Run(root, projection.FrameInfo{})
}

func attachStructureTree(t *testing.T, root facet.FacetImpl) {
	t.Helper()
	facet.Attach(root, facet.AttachContext{})
}

type testWindow struct {
	width   int
	height  int
	surface *memSurface
}

func (w *testWindow) Surface() platform.Surface { return w.surface }
func (w *testWindow) SetTitle(string)           {}
func (w *testWindow) Size() (int, int)          { return w.width, w.height }
func (w *testWindow) ContentScale() float32     { return 1 }
func (w *testWindow) SetIMECursorRect(gfx.Rect) {}
func (w *testWindow) Show()                     {}
func (w *testWindow) Hide()                     {}
func (w *testWindow) Close()                    {}
func (w *testWindow) Destroy()                  {}

type recordingBackend struct {
	last *render.Frame
}

func (b *recordingBackend) Initialize(render.Surface) error { return nil }
func (b *recordingBackend) Submit(frame *render.Frame) error {
	b.last = frame
	return nil
}
func (b *recordingBackend) Resize(int, int) error { return nil }
func (b *recordingBackend) Destroy()              {}

func newStructureRuntime(t *testing.T, root facet.FacetImpl, width, height int) (*runtime.Runtime, *recordingBackend) {
	t.Helper()
	cfg := runtime.DefaultConfig()
	backend := &recordingBackend{}
	window := &testWindow{width: width, height: height, surface: newMemSurface(width, height)}
	rt, err := runtime.New(cfg, nil, window, backend, root)
	if err != nil {
		t.Fatalf("runtime.New: %v", err)
	}
	t.Cleanup(rt.Shutdown)
	return rt, backend
}

type memSurface struct {
	width  int
	height int
	buf    []byte
}

func newMemSurface(width, height int) *memSurface {
	if width < 0 {
		width = 0
	}
	if height < 0 {
		height = 0
	}
	return &memSurface{
		width:  width,
		height: height,
		buf:    make([]byte, width*height*4),
	}
}

func (s *memSurface) Buffer() []byte   { return s.buf }
func (s *memSurface) Stride() int      { return s.width * 4 }
func (s *memSurface) Size() (int, int) { return s.width, s.height }
func (s *memSurface) Scale() float32   { return 1 }
func (s *memSurface) Lock() error      { return nil }
func (s *memSurface) Unlock([]gfx.Rect) error {
	return nil
}
func (s *memSurface) Resize(width, height int) {
	if width < 0 {
		width = 0
	}
	if height < 0 {
		height = 0
	}
	s.width = width
	s.height = height
	s.buf = make([]byte, width*height*4)
}

var _ platform.Surface = (*memSurface)(nil)

func expectPanicContains(t *testing.T, want string, fn func()) {
	t.Helper()
	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatalf("expected panic containing %q", want)
		}
		msg, _ := recovered.(string)
		if want != "" && !strings.Contains(msg, want) {
			t.Fatalf("panic %q missing %q", msg, want)
		}
	}()
	fn()
}

func waitForRenderedFrame(t *testing.T, backend *recordingBackend) *render.Frame {
	t.Helper()
	deadline := time.Now().Add(250 * time.Millisecond)
	for time.Now().Before(deadline) {
		if backend != nil && backend.last != nil {
			return backend.last
		}
		time.Sleep(5 * time.Millisecond)
	}
	if backend != nil && backend.last != nil {
		return backend.last
	}
	t.Fatal("timed out waiting for rendered frame")
	return nil
}
