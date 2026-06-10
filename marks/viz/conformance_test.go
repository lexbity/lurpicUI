package viz

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/scale/reactive"
	"codeburg.org/lexbit/lurpicui/store"
)

// conformanceItem is used by all viz marks that need a data source.
type conformanceItem struct {
	id  store.ItemID
	cat string
	x   float64
	y   float64
}

func confID(i conformanceItem) store.ItemID { return i.id }

// conformanceRuntime satisfies RuntimeServices.
type conformanceRuntime struct{}

func (conformanceRuntime) Schedule(j any)                                                     {}
func (conformanceRuntime) CancelJob(id any)                                                   {}
func (conformanceRuntime) Invalidate(id facet.FacetID, flags facet.DirtyFlags, source string) {}

// registeredMarks returns all viz mark instances to test for conformance.
func registeredMarks(t *testing.T) []marks.Mark {
	t.Helper()
	s := store.NewCollectionStore(confID)
	yDom := store.NewValueStore([2]float64{0, 100})
	yRng := store.NewValueStore([2]float64{0, 300})
	yScale := reactive.NewLinearReactive(yDom, yRng)
	xScale := reactive.NewLinearReactive(yDom, yRng)

	r := NewRule(marks.Const(50.0), RuleHorizontal, yScale)

	a := NewAxis(yScale, marks.Const(AxisBottom), nil)

	p := NewPoint(s,
		func(i conformanceItem) float64 { return i.x },
		func(i conformanceItem) float64 { return i.y },
		xScale, yScale,
	)

	l := NewLine(s,
		func(i conformanceItem) float64 { return i.x },
		func(i conformanceItem) float64 { return i.y },
		xScale, yScale,
	)

	ar := NewArea(s,
		func(i conformanceItem) float64 { return i.x },
		func(i conformanceItem) float64 { return i.y },
		xScale, yScale,
	)

	b := NewBar(s,
		func(i conformanceItem) string { return i.cat },
		func(i conformanceItem) float64 { return i.y },
		yScale,
	)

	return []marks.Mark{r, a, p, l, ar, b}
}

func TestConformance_all_viz_marks_implement_mark(t *testing.T) {
	for _, m := range registeredMarks(t) {
		d := marks.Describe(m)
		if d.Family != "viz" {
			t.Errorf("expected family=viz, got %q", d.Family)
		}
	}
}

func TestConformance_rule_descriptor(t *testing.T) {
	m := NewRule(marks.Const(50.0), RuleHorizontal, nil)
	d := marks.Describe(m)
	if d.TypeName != "rule" {
		t.Errorf("TypeName = %q, want %q", d.TypeName, "rule")
	}
	if !d.ExportsAnchors {
		t.Error("expected ExportsAnchors = true")
	}
}

func TestConformance_axis_descriptor(t *testing.T) {
	s := store.NewValueStore([2]float64{0, 100})
	r := store.NewValueStore([2]float64{0, 300})
	rs := reactive.NewLinearReactive(s, r)
	m := NewAxis(rs, marks.Const(AxisBottom), nil)
	d := marks.Describe(m)
	if d.TypeName != "axis" {
		t.Errorf("TypeName = %q, want %q", d.TypeName, "axis")
	}
	if !d.ExportsAnchors {
		t.Error("expected ExportsAnchors = true")
	}
}

func TestConformance_point_descriptor(t *testing.T) {
	s := store.NewCollectionStore(confID)
	m := NewPoint(s,
		func(i conformanceItem) float64 { return i.x },
		func(i conformanceItem) float64 { return i.y },
		reactive.NewLinearReactive(
			store.NewValueStore([2]float64{0, 1}),
			store.NewValueStore([2]float64{0, 100}),
		),
		reactive.NewLinearReactive(
			store.NewValueStore([2]float64{0, 1}),
			store.NewValueStore([2]float64{0, 100}),
		),
	)
	d := marks.Describe(m)
	if d.TypeName != "point" {
		t.Errorf("TypeName = %q, want %q", d.TypeName, "point")
	}
	if !d.ExportsAnchors {
		t.Error("expected ExportsAnchors = true")
	}
}

func TestConformance_line_descriptor(t *testing.T) {
	s := store.NewCollectionStore(confID)
	m := NewLine(s,
		func(i conformanceItem) float64 { return i.x },
		func(i conformanceItem) float64 { return i.y },
		reactive.NewLinearReactive(
			store.NewValueStore([2]float64{0, 1}),
			store.NewValueStore([2]float64{0, 100}),
		),
		reactive.NewLinearReactive(
			store.NewValueStore([2]float64{0, 1}),
			store.NewValueStore([2]float64{0, 100}),
		),
	)
	d := marks.Describe(m)
	if d.TypeName != "line" {
		t.Errorf("TypeName = %q, want %q", d.TypeName, "line")
	}
	if !d.ExportsAnchors {
		t.Error("expected ExportsAnchors = true")
	}
}

func TestConformance_area_descriptor(t *testing.T) {
	s := store.NewCollectionStore(confID)
	m := NewArea(s,
		func(i conformanceItem) float64 { return i.x },
		func(i conformanceItem) float64 { return i.y },
		reactive.NewLinearReactive(
			store.NewValueStore([2]float64{0, 1}),
			store.NewValueStore([2]float64{0, 100}),
		),
		reactive.NewLinearReactive(
			store.NewValueStore([2]float64{0, 1}),
			store.NewValueStore([2]float64{0, 100}),
		),
	)
	d := marks.Describe(m)
	if d.TypeName != "area" {
		t.Errorf("TypeName = %q, want %q", d.TypeName, "area")
	}
	if !d.ExportsAnchors {
		t.Error("expected ExportsAnchors = true")
	}
}

func TestConformance_bar_descriptor(t *testing.T) {
	s := store.NewCollectionStore(confID)
	m := NewBar(s,
		func(i conformanceItem) string { return i.cat },
		func(i conformanceItem) float64 { return i.y },
		reactive.NewLinearReactive(
			store.NewValueStore([2]float64{0, 1}),
			store.NewValueStore([2]float64{0, 100}),
		),
	)
	d := marks.Describe(m)
	if d.TypeName != "bar" {
		t.Errorf("TypeName = %q, want %q", d.TypeName, "bar")
	}
	if !d.ExportsAnchors {
		t.Error("expected ExportsAnchors = true")
	}
}

func TestConformance_all_marks_export_anchors(t *testing.T) {
	for _, m := range registeredMarks(t) {
		d := marks.Describe(m)
		if !d.ExportsAnchors {
			t.Errorf("%s/%s: expected ExportsAnchors", d.Family, d.TypeName)
		}
	}
}

func TestConformance_all_marks_implement_facet_impl(t *testing.T) {
	for _, m := range registeredMarks(t) {
		base := m.Base()
		if base == nil {
			t.Errorf("%s: Base() returned nil", marks.Describe(m).TypeName)
		}
	}
}

func TestConformance_all_marks_implement_anchor_exporter(t *testing.T) {
	for _, m := range registeredMarks(t) {
		if _, ok := m.(layout.AnchorExporter); !ok {
			d := marks.Describe(m)
			t.Errorf("%s/%s: does not implement layout.AnchorExporter", d.Family, d.TypeName)
		}
	}
}
