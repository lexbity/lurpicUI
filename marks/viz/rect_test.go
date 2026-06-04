package viz

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/scale/reactive"
	"codeburg.org/lexbit/lurpicui/store"
)

type barItem struct {
	id   store.ItemID
	cat  string
	val  float64
}

func barID(i barItem) store.ItemID { return i.id }

func TestBar_produces_fill_rects(t *testing.T) {
	s := store.NewCollectionStore(barID)
	yDom := store.NewValueStore([2]float64{0, 100})
	yRng := store.NewValueStore([2]float64{0, 300})
	yScale := reactive.NewLinearReactive(yDom, yRng)

	b := NewBar(s,
		func(i barItem) string { return i.cat },
		func(i barItem) float64 { return i.val },
		yScale,
	)
	facet.Attach(b, facet.AttachContext{Runtime: vizRuntimeStub{}})
	b.OnAttach(facet.AttachContext{Runtime: vizRuntimeStub{}})

	s.Insert(barItem{id: 1, cat: "A", val: 50})
	s.Insert(barItem{id: 2, cat: "B", val: 80})
	s.Insert(barItem{id: 3, cat: "C", val: 30})

	bounds := gfx.RectFromXYWH(10, 10, 300, 300)
	b.Layout.Arrange(facet.ArrangeContext{}, bounds)

	cmds := b.Projection.Project(facet.ProjectionContext{Bounds: bounds})
	if cmds == nil || len(cmds.Commands) == 0 {
		t.Fatal("expected projection commands")
	}

	fr, ok := cmds.Commands[0].(gfx.FillRect)
	if !ok {
		t.Fatalf("expected FillRect, got %T", cmds.Commands[0])
	}
	if fr.Rect.Width() <= 0 || fr.Rect.Height() <= 0 {
		t.Fatal("expected bar with positive dimensions")
	}
}

func TestBar_empty_store(t *testing.T) {
	s := store.NewCollectionStore(barID)
	b := NewBar(s,
		func(i barItem) string { return i.cat },
		func(i barItem) float64 { return i.val },
		reactive.NewLinearReactive(
			store.NewValueStore([2]float64{0, 1}),
			store.NewValueStore([2]float64{0, 100}),
		),
	)
	facet.Attach(b, facet.AttachContext{Runtime: vizRuntimeStub{}})
	b.OnAttach(facet.AttachContext{Runtime: vizRuntimeStub{}})

	bounds := gfx.RectFromXYWH(0, 0, 100, 100)
	b.Layout.Arrange(facet.ArrangeContext{}, bounds)
	cmds := b.Projection.Project(facet.ProjectionContext{Bounds: bounds})
	if cmds != nil {
		t.Fatal("expected nil commands for empty store")
	}
}

func TestBar_bar_count_matches_data(t *testing.T) {
	s := store.NewCollectionStore(barID)
	yDom := store.NewValueStore([2]float64{0, 100})
	yRng := store.NewValueStore([2]float64{0, 300})
	yScale := reactive.NewLinearReactive(yDom, yRng)

	b := NewBar(s,
		func(i barItem) string { return i.cat },
		func(i barItem) float64 { return i.val },
		yScale,
	)
	facet.Attach(b, facet.AttachContext{Runtime: vizRuntimeStub{}})
	b.OnAttach(facet.AttachContext{Runtime: vizRuntimeStub{}})

	s.Insert(barItem{id: 1, cat: "A", val: 10})
	s.Insert(barItem{id: 2, cat: "B", val: 20})
	s.Insert(barItem{id: 3, cat: "C", val: 30})
	s.Insert(barItem{id: 4, cat: "D", val: 40})

	bounds := gfx.RectFromXYWH(0, 0, 200, 200)
	b.Layout.Arrange(facet.ArrangeContext{}, bounds)

	cmds := b.Projection.Project(facet.ProjectionContext{Bounds: bounds})
	if len(cmds.Commands) != 4 {
		t.Fatalf("expected 4 bars, got %d", len(cmds.Commands))
	}
}

func TestBar_hit_test_returns_member(t *testing.T) {
	s := store.NewCollectionStore(barID)
	yDom := store.NewValueStore([2]float64{0, 100})
	yRng := store.NewValueStore([2]float64{0, 300})
	yScale := reactive.NewLinearReactive(yDom, yRng)

	b := NewBar(s,
		func(i barItem) string { return i.cat },
		func(i barItem) float64 { return i.val },
		yScale,
	)
	facet.Attach(b, facet.AttachContext{Runtime: vizRuntimeStub{}})
	b.OnAttach(facet.AttachContext{Runtime: vizRuntimeStub{}})

	s.Insert(barItem{id: 1, cat: "A", val: 50})
	s.Insert(barItem{id: 2, cat: "B", val: 80})

	bounds := gfx.RectFromXYWH(0, 0, 200, 200)
	b.Layout.Arrange(facet.ArrangeContext{}, bounds)
	b.Projection.Project(facet.ProjectionContext{Bounds: bounds})

	// Hit-test inside the first bar
	member, ok := b.HitMember(gfx.Point{X: 5, Y: 100})
	if !ok {
		t.Fatal("expected hit on first bar")
	}
	if member != "A" {
		t.Fatalf("hit member = %q, want %q", member, "A")
	}
}

func TestBar_hit_test_misses_gap(t *testing.T) {
	s := store.NewCollectionStore(barID)
	yDom := store.NewValueStore([2]float64{0, 100})
	yRng := store.NewValueStore([2]float64{0, 300})
	yScale := reactive.NewLinearReactive(yDom, yRng)

	b := NewBar(s,
		func(i barItem) string { return i.cat },
		func(i barItem) float64 { return i.val },
		yScale,
	)
	b.Padding = marks.Const[float32](0.3) // creates gaps between bars
	facet.Attach(b, facet.AttachContext{Runtime: vizRuntimeStub{}})
	b.OnAttach(facet.AttachContext{Runtime: vizRuntimeStub{}})

	s.Insert(barItem{id: 1, cat: "A", val: 50})
	s.Insert(barItem{id: 2, cat: "B", val: 80})

	bounds := gfx.RectFromXYWH(0, 0, 200, 200)
	b.Layout.Arrange(facet.ArrangeContext{}, bounds)
	b.Projection.Project(facet.ProjectionContext{Bounds: bounds})

	// Try to hit in the gap area between bars
	if len(b.barRects) >= 2 {
		// Gap is between rect[0].Max.X and rect[1].Min.X
		gapX := (b.barRects[0].Max.X + b.barRects[1].Min.X) / 2
		_, ok := b.HitMember(gfx.Point{X: gapX, Y: 100})
		if ok {
			t.Fatal("expected miss in the gap between bars")
		}
	}
}
