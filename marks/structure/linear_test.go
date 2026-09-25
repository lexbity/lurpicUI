package structure

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/layout/linear"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/action"
	"codeburg.org/lexbit/lurpicui/marks/contracttest"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uiinput"
)

// fixedFacet is a bare facet with a constant measured size, so layout math
// tests get exact numbers instead of text-metric approximations.
type fixedFacet struct {
	facet.Facet

	layout facet.LayoutRole
	size   gfx.Size
}

func newFixedFacet(w, h float32) *fixedFacet {
	f := &fixedFacet{Facet: facet.NewFacet(), size: gfx.Size{W: w, H: h}}
	f.layout.OnMeasure = func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
		return facet.MeasureResult{Size: facet.Constraints{MaxSize: c.MaxSize}.Constrain(f.size)}
	}
	f.layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		f.layout.ArrangedBounds = bounds
	}
	f.layout.Child = facet.GroupChildContract{
		SupportedPlacement: facet.SupportsLinear | facet.SupportsGrid | facet.SupportsAnchor | facet.SupportsFree,
		Stretch: facet.StretchPolicy{
			Width:  facet.StretchNever,
			Height: facet.StretchNever,
		},
	}
	f.AddRole(&f.layout)
	return f
}

func (f *fixedFacet) Base() *facet.Facet {
	f.BindImpl(f)
	return &f.Facet
}

func measureCtx() facet.MeasureContext {
	return facet.MeasureContext{Theme: theme.DefaultResolvedContext()}
}

// childBoundsByID collects arranged bounds for the row's declared children.
func arrangedBounds(t *testing.T, kids ...facet.FacetImpl) map[facet.FacetImpl]gfx.Rect {
	t.Helper()
	out := make(map[facet.FacetImpl]gfx.Rect, len(kids))
	for _, k := range kids {
		b := k.Base()
		if b == nil {
			t.Fatal("child facet base is nil")
		}
		out[k] = b.LayoutRole().ArrangedBounds
	}
	return out
}

func TestRow_measures_children_and_sums_with_gaps(t *testing.T) {
	c1 := newFixedFacet(40, 20)
	c2 := newFixedFacet(60, 30)

	r := NewRow(
		[]AxisChild{
			{Facet: c1},
			{Facet: c2},
		},
		AxisConfig{Gap: 10, PadX: 4, PadY: 2},
	)

	result := r.measure(measureCtx(), facet.Constraints{MaxSize: gfx.Size{W: 400, H: 100}})
	// main = 40 + 60 + gap 10 = 110; +padX*2 = 118. cross = max(20,30)=30; +padY*2 = 34.
	if result.Size.W != 118 || result.Size.H != 34 {
		t.Fatalf("expected measured 118x34, got %#v", result.Size)
	}
}

func TestColumn_measures_children_and_sums_with_gaps(t *testing.T) {
	c1 := newFixedFacet(40, 20)
	c2 := newFixedFacet(60, 30)

	c := NewColumn(
		[]AxisChild{
			{Facet: c1},
			{Facet: c2},
		},
		AxisConfig{Gap: 10, PadX: 4, PadY: 2},
	)

	result := c.measure(measureCtx(), facet.Constraints{MaxSize: gfx.Size{W: 400, H: 400}})
	// main = 20 + 30 + gap 10 = 60; +padY*2 = 64. cross = max(40,60)=60; +padX*2 = 68.
	if result.Size.W != 68 || result.Size.H != 64 {
		t.Fatalf("expected measured 68x64, got %#v", result.Size)
	}
}

func TestRow_arranges_children_in_order_with_weighted_fill(t *testing.T) {
	hug := newFixedFacet(50, 20)
	w1 := newFixedFacet(10, 20)
	w2 := newFixedFacet(10, 20)

	r := NewRow(
		[]AxisChild{
			{Facet: hug, Weight: 0},
			{Facet: w1, Weight: 1},
			{Facet: w2, Weight: 2},
		},
		AxisConfig{},
	)

	_ = r.measure(measureCtx(), facet.Constraints{MaxSize: gfx.Size{W: 310, H: 50}})
	r.arrange(facet.ArrangeContext{}, gfx.RectFromXYWH(0, 0, 310, 50))

	bounds := arrangedBounds(t, hug, w1, w2)
	// residual = 310 - 70 = 240, shared 80/160 proportional to weights 1:2,
	// added on top of each child's measured main size (10).
	hb := bounds[hug]
	b1 := bounds[w1]
	b2 := bounds[w2]
	if hb.Min.X != 0 || hb.Width() != 50 {
		t.Fatalf("hug child expected [0,50], got %#v", hb)
	}
	if b1.Min.X != 50 || b1.Width() != 90 {
		t.Fatalf("weight-1 child expected [50,140], got %#v", b1)
	}
	if b2.Min.X != 140 || b2.Width() != 170 {
		t.Fatalf("weight-2 child expected [140,310], got %#v", b2)
	}
}

func TestRow_unweighted_fill_children_share_equally(t *testing.T) {
	w1 := newFixedFacet(10, 20)
	w2 := newFixedFacet(10, 20)
	w3 := newFixedFacet(10, 20)

	r := NewRow(
		[]AxisChild{
			{Facet: w1, Weight: 1},
			{Facet: w2, Weight: 1},
			{Facet: w3, Weight: 1},
		},
		AxisConfig{},
	)

	_ = r.measure(measureCtx(), facet.Constraints{MaxSize: gfx.Size{W: 300, H: 50}})
	r.arrange(facet.ArrangeContext{}, gfx.RectFromXYWH(0, 0, 300, 50))

	bounds := arrangedBounds(t, w1, w2, w3)
	// residual = 300 - 30 = 270 → 90 each, added on top of measured 10 → 100.
	for i, k := range []facet.FacetImpl{w1, w2, w3} {
		b := bounds[k]
		wantMin := float32(i * 100)
		if b.Min.X != wantMin || b.Width() != 100 {
			t.Fatalf("child %d expected [%v,%v] wide 100, got %#v", i, wantMin, wantMin+100, b)
		}
	}
}

func TestRow_hug_child_stays_at_intrinsic_width(t *testing.T) {
	hug := newFixedFacet(70, 20)
	fill := newFixedFacet(10, 20)

	r := NewRow(
		[]AxisChild{
			{Facet: hug, Weight: 0},
			{Facet: fill, Weight: 1},
		},
		AxisConfig{},
	)

	_ = r.measure(measureCtx(), facet.Constraints{MaxSize: gfx.Size{W: 200, H: 50}})
	r.arrange(facet.ArrangeContext{}, gfx.RectFromXYWH(0, 0, 200, 50))

	bounds := arrangedBounds(t, hug, fill)
	if got := bounds[hug].Width(); got != 70 {
		t.Fatalf("hug child width = %v, want 70", got)
	}
	if got := bounds[fill].Width(); got != 130 {
		t.Fatalf("fill child width = %v, want 130 (200-70)", got)
	}
}

func TestRow_cross_align_positions_children(t *testing.T) {
	start := newFixedFacet(30, 20)
	center := newFixedFacet(30, 20)
	end := newFixedFacet(30, 20)

	r := NewRow(
		[]AxisChild{
			{Facet: start, Align: CrossAlignStart},
			{Facet: center, Align: CrossAlignCenter},
			{Facet: end, Align: CrossAlignEnd},
		},
		AxisConfig{},
	)

	_ = r.measure(measureCtx(), facet.Constraints{MaxSize: gfx.Size{W: 200, H: 100}})
	r.arrange(facet.ArrangeContext{}, gfx.RectFromXYWH(0, 0, 200, 100))

	bounds := arrangedBounds(t, start, center, end)
	if got := bounds[start].Min.Y; got != 0 {
		t.Fatalf("start-aligned child Min.Y = %v, want 0", got)
	}
	if got := bounds[center].Min.Y; got != 40 {
		t.Fatalf("center-aligned child Min.Y = %v, want 40", got)
	}
	if got := bounds[end].Max.Y; got != 100 {
		t.Fatalf("end-aligned child Max.Y = %v, want 100", got)
	}
}

func TestColumn_arranges_children_top_to_bottom(t *testing.T) {
	top := newFixedFacet(50, 30)
	bottom := newFixedFacet(50, 40)

	c := NewColumn(
		[]AxisChild{
			{Facet: top},
			{Facet: bottom},
		},
		AxisConfig{Gap: 10},
	)

	_ = c.measure(measureCtx(), facet.Constraints{MaxSize: gfx.Size{W: 200, H: 300}})
	c.arrange(facet.ArrangeContext{}, gfx.RectFromXYWH(0, 0, 200, 300))

	bounds := arrangedBounds(t, top, bottom)
	tb := bounds[top]
	bb := bounds[bottom]
	if tb.Min.Y != 0 || tb.Height() != 30 {
		t.Fatalf("top child expected [0,30], got %#v", tb)
	}
	if bb.Min.Y != 40 || bb.Height() != 40 {
		t.Fatalf("bottom child expected [40,80], got %#v", bb)
	}
	// Neither child requested fill, so neither should absorb the free space.
	if total := bounds[bottom].Max.Y; total != 80 {
		t.Fatalf("children should hug (total 80), got %v", total)
	}
}

// rowTestRuntime is the minimal runtime the Button mark discovers via type
// assertion for font and style resolution during measure.
type rowTestRuntime struct {
	contracttest.NoopRuntime
	fonts *text.FontRegistry
}

func (s rowTestRuntime) FontRegistry() *text.FontRegistry { return s.fonts }

func (rowTestRuntime) RootStyleContext() any {
	return theme.NewRootStyleContext(nil, theme.DefaultTokens(), nil)
}

func TestRow_hosts_interactive_child(t *testing.T) {
	btn := action.NewButton(marks.Const("Save"), marks.Const(uiinput.ButtonFilled))

	r := NewRow([]AxisChild{{Facet: btn, Weight: 1}}, AxisConfig{})
	r.OnAttach(facet.AttachContext{})

	// The button must be a real tree child of the row.
	found := false
	for _, ch := range r.Base().Children() {
		if ch.ID() == btn.Base().ID() {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("Row did not attach the button as a tree child")
	}

	ctx := measureCtx()
	ctx.Runtime = rowTestRuntime{fonts: testkit.TestFontRegistry(t)}
	_ = r.measure(ctx, facet.Constraints{MaxSize: gfx.Size{W: 300, H: 60}})
	r.arrange(facet.ArrangeContext{}, gfx.RectFromXYWH(0, 0, 300, 60))

	// A click at the button's arranged center must hit the button — proving
	// the hit region derives from the arranged bounds, not self-projection.
	b := btn.Base().LayoutRole().ArrangedBounds
	if b.IsEmpty() {
		t.Fatalf("button has empty arranged bounds; measure result %#v", btn.Base().LayoutRole().MeasuredSize)
	}
	center := gfx.Point{X: (b.Min.X + b.Max.X) / 2, Y: (b.Min.Y + b.Max.Y) / 2}
	if hit := btn.Hit.HitTest(center); !hit.Hit {
		t.Fatalf("button not hit at arranged center %#v (bounds %#v)", center, b)
	}

	// A point outside the button's arranged bounds must not hit it.
	outside := gfx.Point{X: b.Max.X + 10, Y: b.Max.Y + 10}
	if hit := btn.Hit.HitTest(outside); hit.Hit {
		t.Fatalf("button hit outside its arranged bounds at %#v", outside)
	}

	r.OnDetach()
}

func TestRow_gap_binding_change_remasures(t *testing.T) {
	c1 := newFixedFacet(40, 20)
	c2 := newFixedFacet(60, 20)

	gapStore := store.NewValueStore(float32(0))
	r := NewRow(
		[]AxisChild{
			{Facet: c1},
			{Facet: c2},
		},
		AxisConfig{},
	)
	// Gap is dynamic here (a store-backed binding), so it is assigned after
	// construction — the sanctioned post-ctor form for content sources.
	r.Gap = marks.FromStore(gapStore, facet.DirtyLayout)

	constraints := facet.Constraints{MaxSize: gfx.Size{W: 400, H: 50}}
	_ = r.measure(measureCtx(), constraints)
	w1 := r.Layout.MeasuredSize.W

	gapStore.Set(float32(20))
	_ = r.measure(measureCtx(), constraints)
	w2 := r.Layout.MeasuredSize.W

	if w2 != w1+20 {
		t.Fatalf("expected width to grow by gap delta: %v -> %v", w1, w2)
	}
}

func TestRow_empty_measures_to_padding(t *testing.T) {
	r := NewRow(nil, AxisConfig{PadX: 4, PadY: 6})
	result := r.measure(measureCtx(), facet.Constraints{MaxSize: gfx.Size{W: 100, H: 100}})
	if result.Size.W != 8 || result.Size.H != 12 {
		t.Fatalf("expected 8x12 for empty row, got %#v", result.Size)
	}
	if len(r.Children()) != 0 {
		t.Fatalf("expected 0 children for empty row, got %d", len(r.Children()))
	}
}

func TestDivider_horizontal_by_default(t *testing.T) {
	d := NewDivider()
	d.Thickness = marks.Const(float32(2))

	result := d.measure(measureCtx(), facet.Constraints{MaxSize: gfx.Size{W: 200, H: 10}})
	if result.Size.H != 2 {
		t.Fatalf("expected horizontal divider height 2, got %#v", result.Size)
	}
	if result.Size.W != 200 {
		t.Fatalf("expected horizontal divider to fill width, got %#v", result.Size)
	}

	d.Layout.Arrange(facet.ArrangeContext{}, gfx.RectFromXYWH(0, 0, 200, 2))
	cmds := d.buildCommands(d.Layout.ArrangedBounds)
	if len(cmds) == 0 {
		t.Fatal("expected draw commands for an explicitly colored divider")
	}
}

func TestDivider_vertical_inside_row(t *testing.T) {
	d := NewDivider()
	d.Thickness = marks.Const(float32(2))
	d.Color = marks.Const(gfx.Color{R: 0, G: 0, B: 0, A: 1})

	r := NewRow(
		[]AxisChild{
			{Facet: newFixedFacet(40, 20)},
			{Facet: d},
			{Facet: newFixedFacet(40, 20)},
		},
		AxisConfig{},
	)
	r.OnAttach(facet.AttachContext{})

	_ = r.measure(measureCtx(), facet.Constraints{MaxSize: gfx.Size{W: 200, H: 20}})
	// A divider in a Row is a vertical stroke: thickness wide, cross-height.
	if got := d.Layout.MeasuredSize.W; got != 2 {
		t.Fatalf("expected vertical divider width 2 inside a Row, got %v", got)
	}

	r.arrange(facet.ArrangeContext{}, gfx.RectFromXYWH(0, 0, 200, 20))
	bounds := d.Base().LayoutRole().ArrangedBounds
	if bounds.Width() != 2 {
		t.Fatalf("expected arranged vertical stroke width 2, got %#v", bounds)
	}
	cmds := d.buildCommands(bounds)
	if len(cmds) == 0 {
		t.Fatal("expected draw commands for in-row divider")
	}
	r.OnDetach()
}

func TestDivider_horizontal_inside_column(t *testing.T) {
	d := NewDivider()
	d.Thickness = marks.Const(float32(3))
	d.Color = marks.Const(gfx.Color{R: 0, G: 0, B: 0, A: 1})

	c := NewColumn(
		[]AxisChild{
			{Facet: newFixedFacet(40, 20)},
			{Facet: d},
		},
		AxisConfig{},
	)
	c.OnAttach(facet.AttachContext{})

	_ = c.measure(measureCtx(), facet.Constraints{MaxSize: gfx.Size{W: 200, H: 100}})
	if got := d.Layout.MeasuredSize.H; got != 3 {
		t.Fatalf("expected horizontal divider height 3 inside a Column, got %v", got)
	}
	c.OnDetach()
}

func TestDivider_default_color_draws_from_theme(t *testing.T) {
	d := NewDivider() // Color left as zero → themed default
	d.Thickness = marks.Const(float32(1))
	d.measure(measureCtx(), facet.Constraints{MaxSize: gfx.Size{W: 100, H: 1}})
	d.Layout.Arrange(facet.ArrangeContext{}, gfx.RectFromXYWH(0, 0, 100, 1))
	cmds := d.buildCommands(d.Layout.ArrangedBounds)
	if len(cmds) == 0 {
		t.Fatal("expected themed default divider to draw without an explicit Color")
	}
}

func TestDescriptors(t *testing.T) {
	cases := []struct {
		desc marks.Descriptor
		want marks.Descriptor
	}{
		{NewRow(nil, AxisConfig{}).Descriptor(), marks.Descriptor{Family: "structure", TypeName: "row"}},
		{NewColumn(nil, AxisConfig{}).Descriptor(), marks.Descriptor{Family: "structure", TypeName: "column"}},
		{NewDivider().Descriptor(), marks.Descriptor{Family: "structure", TypeName: "divider"}},
	}
	for i, tc := range cases {
		if tc.desc != tc.want {
			t.Errorf("case %d: descriptor = %v, want %v", i, tc.desc, tc.want)
		}
	}
}

func TestAccessibilityRoles(t *testing.T) {
	if got := NewRow(nil, AxisConfig{}).AccessibilityRole(); got != "group" {
		t.Errorf("row role = %q, want group", got)
	}
	if got := NewColumn(nil, AxisConfig{}).AccessibilityRole(); got != "group" {
		t.Errorf("column role = %q, want group", got)
	}
	if got := NewDivider().AccessibilityRole(); got != "separator" {
		t.Errorf("divider role = %q, want separator", got)
	}
}

// Compile-time guard: the linear placement weight flows into the policy via
// groupChildToLinear; keep the conversion honest if fields change.
var _ = linear.Child{}
