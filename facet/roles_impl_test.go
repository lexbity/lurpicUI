package facet

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/store"
)

func TestLayoutRole_measure_called_with_constraints(t *testing.T) {
	role := &LayoutRole{
		OnMeasure: func(c Constraints) gfx.Size {
			if c.MinSize != (gfx.Size{W: 10, H: 20}) || c.MaxSize != (gfx.Size{W: 30, H: 40}) {
				t.Fatalf("unexpected constraints: %#v", c)
			}
			return gfx.Size{W: 11, H: 22}
		},
	}
	got := role.Measure(Constraints{
		MinSize: gfx.Size{W: 10, H: 20},
		MaxSize: gfx.Size{W: 30, H: 40},
	})
	if got != (gfx.Size{W: 11, H: 22}) {
		t.Fatalf("got %#v", got)
	}
	if role.MeasuredSize != got {
		t.Fatalf("cached size %#v", role.MeasuredSize)
	}
}

func TestLayoutRole_missing_onmeasure_panics_on_attach(t *testing.T) {
	f := &Facet{state: StateCreated}
	role := &LayoutRole{}
	f.roles = []Role{role}
	mustPanic(t, func() { Attach(f, AttachContext{}) })
}

func TestRenderRole_collect_called_with_bounds(t *testing.T) {
	role := &RenderRole{
		OnCollect: func(list *gfx.CommandList, bounds gfx.Rect) {
			if bounds != (gfx.Rect{Min: gfx.Point{X: 1, Y: 2}, Max: gfx.Point{X: 3, Y: 4}}) {
				t.Fatalf("unexpected bounds: %#v", bounds)
			}
			list.Add(gfx.FillRect{Rect: bounds})
		},
	}
	list := role.Collect(gfx.Rect{Min: gfx.Point{X: 1, Y: 2}, Max: gfx.Point{X: 3, Y: 4}})
	if list == nil || list.Len() != 1 {
		t.Fatalf("expected one command, got %#v", list)
	}
}

func TestRenderRole_layerid_assigned_on_attach(t *testing.T) {
	f := &Facet{state: StateCreated}
	role := &RenderRole{}
	f.roles = []Role{role}
	Attach(f, AttachContext{})
	if role.LayerID == 0 {
		t.Fatal("expected non-zero layer id")
	}
}

func TestHitRole_hittest_called_with_local_point(t *testing.T) {
	role := &HitRole{
		OnHitTest: func(p gfx.Point) HitResult {
			if p != (gfx.Point{X: 7, Y: 9}) {
				t.Fatalf("unexpected point: %#v", p)
			}
			return HitResult{Hit: true, MarkID: 7, Cursor: CursorPointer}
		},
	}
	got := role.HitTest(gfx.Point{X: 7, Y: 9})
	if !got.Hit || got.MarkID != 7 || got.Cursor != CursorPointer {
		t.Fatalf("got %#v", got)
	}
}

func TestHitRole_missing_onhittest_panics_on_attach(t *testing.T) {
	f := &Facet{state: StateCreated}
	role := &HitRole{}
	f.roles = []Role{role}
	mustPanic(t, func() { Attach(f, AttachContext{}) })
}

func TestProjectionRole_missing_onproject_panics_on_attach(t *testing.T) {
	f := &Facet{state: StateCreated}
	role := &ProjectionRole{}
	f.roles = []Role{role}
	mustPanic(t, func() { Attach(f, AttachContext{}) })
}

func TestTrackStore_registers_version_source(t *testing.T) {
	f := NewFacet()
	v := store.NewValueStore(1)
	TrackStore(f.Subs(), &f.subscribedVersions, v.Version, &v.OnChange, func(signal.Change[int]) {})
	got := f.SubscribedVersions()
	if len(got) != 1 || got[0] != v.Version() {
		t.Fatalf("got %#v want [%d]", got, v.Version())
	}
}

func TestFacetSubs_released_on_dispose(t *testing.T) {
	f := &Facet{state: StateCreated, id: nextID()}
	role := &LayoutRole{OnMeasure: func(Constraints) gfx.Size { return gfx.Size{W: 1, H: 1} }}
	f.roles = []Role{role}
	sig := signal.NewSignal[signal.Unit]("test")
	signal.Track(f.Subs(), &sig, func(signal.Unit) {})
	if got := f.Subs().Len(); got != 1 {
		t.Fatalf("expected 1 sub, got %d", got)
	}
	Attach(f, AttachContext{})
	Dispose(f)
	if got := f.Subs().Len(); got != 0 {
		t.Fatalf("expected released subs, got %d", got)
	}
}
