package viz

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/scale/reactive"
)

// RuleOrientation describes whether the rule spans horizontally or vertically.
type RuleOrientation uint8

const (
	RuleHorizontal RuleOrientation = iota
	RuleVertical
)

// Rule is a reference line at a given domain value.
type Rule struct {
	marks.Core

	Value       marks.Binding[float64]
	Orientation RuleOrientation
	Scale       *reactive.ReactiveScale
	Color       gfx.Color
	StrokeWidth float32
}

var _ facet.FacetImpl = (*Rule)(nil)
var _ layout.AnchorExporter = (*Rule)(nil)
var _ marks.Mark = (*Rule)(nil)

// NewRule constructs a reference line mark.
func NewRule(value marks.Binding[float64], orientation RuleOrientation, scale *reactive.ReactiveScale) *Rule {
	r := &Rule{
		Value:       value,
		Orientation: orientation,
		Scale:       scale,
		Color:       gfx.Color{R: 0.7, G: 0.7, B: 0.7, A: 1},
		StrokeWidth: 1,
	}
	r.Core.Facet = facet.NewFacet()
	r.AddBinding(r.Value)

	r.Layout.OnMeasure = func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
		return facet.MeasureResult{Size: gfx.Size{W: 0, H: 0}}
	}
	r.Layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		r.Layout.ArrangedBounds = bounds
	}
	r.BuildCommands = func(ctx facet.ProjectionContext) []gfx.Command {
		return r.buildCommands(r.Layout.ArrangedBounds)
	}
	r.RegisterRoles()
	return r
}

func (r *Rule) Base() *facet.Facet {
	r.Facet.BindImpl(r)
	return &r.Facet
}

func (r *Rule) Descriptor() marks.Descriptor {
	return marks.Descriptor{Family: "viz", TypeName: "rule"}
}

func (r *Rule) OnAttach(ctx facet.AttachContext)  { r.Core.OnAttach() }
func (r *Rule) OnDetach()                          { r.Core.OnDetach() }
func (r *Rule) OnActivate()                        { r.Core.OnActivate() }
func (r *Rule) OnDeactivate()                      { r.Core.OnDeactivate() }

func (r *Rule) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	return r.DefaultAnchors(r.Layout.ArrangedBounds, ctx)
}

func (r *Rule) buildCommands(bounds gfx.Rect) []gfx.Command {
	if bounds.IsEmpty() || r.Scale == nil {
		return nil
	}
	pixel := r.Scale.Get().Map(r.Value.Get())
	return []gfx.Command{
		gfx.StrokePath{
			Path:   r.linePath(bounds, float32(pixel)),
			Stroke: gfx.StrokeStyle{Width: r.StrokeWidth},
			Brush:  gfx.SolidBrush(r.Color),
		},
	}
}

func (r *Rule) linePath(bounds gfx.Rect, pos float32) gfx.Path {
	if r.Orientation == RuleHorizontal {
		y := bounds.Min.Y + pos
		return gfx.Path{Segments: []gfx.PathSegment{
			{Verb: gfx.PathMoveTo, Pts: [3]gfx.Point{{X: bounds.Min.X, Y: y}}},
			{Verb: gfx.PathLineTo, Pts: [3]gfx.Point{{X: bounds.Max.X, Y: y}}},
		}}
	}
	x := bounds.Min.X + pos
	return gfx.Path{Segments: []gfx.PathSegment{
		{Verb: gfx.PathMoveTo, Pts: [3]gfx.Point{{X: x, Y: bounds.Min.Y}}},
		{Verb: gfx.PathLineTo, Pts: [3]gfx.Point{{X: x, Y: bounds.Max.Y}}},
	}}
}
