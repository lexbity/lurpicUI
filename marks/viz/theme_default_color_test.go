package viz

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/scale/reactive"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/theme"
)

// TestLineDefaultColor_ResolvedFromTheme is the regression guard for the
// theme-color placement bug: a viz mark whose Color binding is NOT set
// explicitly must resolve its color from the theme context threaded through
// the layout pass, NOT render as a zero (transparent/black) default.
//
// The original bug synced theme in OnAttach (where AttachContext.Theme is nil),
// so the default color never resolved and an unset Line rendered invisible.
// This test drives the layout callbacks with a themed MeasureContext/
// ArrangeContext (exactly what the runtime does at runtime/layout.go:188,207)
// and asserts the rendered line is the theme's primary color, not blank.
//
// If syncThemeColor is ever moved back to OnAttach, or the OnMeasure/OnArrange
// callbacks stop calling it, this golden flips to a blank surface and fails.
func TestLineDefaultColor_ResolvedFromTheme(t *testing.T) {
	s := store.NewCollectionStore(scatterID)
	xDom := store.NewValueStore([2]float64{0, 10})
	xRng := store.NewValueStore([2]float64{0, 300})
	yDom := store.NewValueStore([2]float64{0, 10})
	yRng := store.NewValueStore([2]float64{0, 300})
	xScale := reactive.NewLinearReactive(xDom, xRng)
	yScale := reactive.NewLinearReactive(yDom, yRng)

	l := NewLine(s,
		func(i scatterItem) float64 { return i.x },
		func(i scatterItem) float64 { return i.y },
		xScale, yScale,
	)
	l.StrokeWidth = marks.Const[float32](2)
	// Deliberately do NOT set l.Color — leave the gfx.Color{} default so the
	// theme-resolution path is the only source of a visible color.

	facet.Attach(l, facet.AttachContext{Runtime: vizRuntimeStub{}})
	l.OnAttach(facet.AttachContext{Runtime: vizRuntimeStub{}})

	s.Insert(scatterItem{id: 1, x: 5, y: 5})

	// Drive the layout pass with a populated Theme, exactly as the runtime does.
	themeCtx := theme.DefaultResolvedContext()
	bounds := gfx.RectFromXYWH(20, 20, 300, 300)
	l.Layout.Measure(facet.MeasureContext{Theme: themeCtx}, facet.Constraints{
		MaxSize: gfx.Size{W: 340, H: 340},
	})
	l.Layout.Arrange(facet.ArrangeContext{Theme: themeCtx}, bounds)

	// The mark must have resolved a non-zero default color from the theme.
	if l.themeColor == (gfx.Color{}) {
		t.Fatalf("themeColor is zero after a themed layout pass; the default-color " +
			"path is broken — a Line without an explicit Color would render invisible")
	}
	wantColor := themeCtx.Color(theme.ColorPrimary)
	if l.themeColor != wantColor {
		t.Fatalf("themeColor = %v, want resolved %v", l.themeColor, wantColor)
	}

	proj := l.Projection.Project(facet.ProjectionContext{Bounds: bounds})
	if proj == nil || len(proj.Commands) == 0 {
		t.Fatal("expected projected commands for the default-colored line, got none")
	}

	surface := renderAxisGolden(t, proj.Commands, bounds, 340, 340)
	testkit.AssertGolden(t, surface, "line_default_color")
}

// TestRuleDefaultColor_ResolvedFromTheme mirrors the Line guard for Rule,
// which resolves a neutral ColorBorder token (not primary) and has zero
// natural size. It pins that the guide line is theme-tinted, not black.
func TestRuleDefaultColor_ResolvedFromTheme(t *testing.T) {
	dom := store.NewValueStore([2]float64{0, 10})
	rng := store.NewValueStore([2]float64{0, 300})
	scale := reactive.NewLinearReactive(dom, rng)

	r := NewRule(marks.Const(5.0), RuleHorizontal, scale)
	// Deliberately do NOT set r.Color.

	facet.Attach(r, facet.AttachContext{Runtime: vizRuntimeStub{}})
	r.OnAttach(facet.AttachContext{Runtime: vizRuntimeStub{}})

	themeCtx := theme.DefaultResolvedContext()
	bounds := gfx.RectFromXYWH(20, 20, 300, 300)
	r.Layout.Measure(facet.MeasureContext{Theme: themeCtx}, facet.Constraints{
		MaxSize: gfx.Size{W: 340, H: 340},
	})
	r.Layout.Arrange(facet.ArrangeContext{Theme: themeCtx}, bounds)

	if r.themeColor == (gfx.Color{}) {
		t.Fatalf("themeColor is zero after a themed layout pass; the default-color " +
			"path is broken — a Rule without an explicit Color would render invisible")
	}
	wantColor := themeCtx.Color(theme.ColorBorder)
	if r.themeColor != wantColor {
		t.Fatalf("themeColor = %v, want resolved %v", r.themeColor, wantColor)
	}
}
