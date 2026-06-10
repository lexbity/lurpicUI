package viz

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/scale/reactive"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/store"
)

// Point is a data-bound scatter mark that renders points through x/y scales.
type Point[T any] struct {
	marks.Core

	Store  *store.CollectionStore[T]
	X, Y   func(T) float64
	XScale *reactive.ReactiveScale
	YScale *reactive.ReactiveScale

	Radius marks.Binding[float32]
	Color  gfx.Color

	cleanups []func()
}

var _ facet.FacetImpl = (*Point[int])(nil)
var _ layout.AnchorExporter = (*Point[int])(nil)
var _ marks.Mark = (*Point[int])(nil)

// NewPoint constructs a scatter mark.
func NewPoint[T any](
	store *store.CollectionStore[T],
	x, y func(T) float64,
	xScale, yScale *reactive.ReactiveScale,
) *Point[T] {
	p := &Point[T]{
		Store:  store,
		X:      x,
		Y:      y,
		XScale: xScale,
		YScale: yScale,
		Radius: marks.Const[float32](4),
		Color:  gfx.Color{R: 0.2, G: 0.4, B: 0.8, A: 1},
	}
	p.Facet = facet.NewFacet()
	p.AddBinding(p.Radius)

	p.Layout.OnMeasure = func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
		return facet.MeasureResult{Size: constraints.MaxSize}
	}
	p.Layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		p.Layout.ArrangedBounds = bounds
	}
	p.BuildCommands = func(ctx facet.ProjectionContext) []gfx.Command {
		return p.buildCommands(p.Layout.ArrangedBounds)
	}
	p.RegisterRoles()
	return p
}

func (p *Point[T]) Base() *facet.Facet {
	p.BindImpl(p)
	return &p.Facet
}

func (p *Point[T]) Descriptor() marks.Descriptor {
	return marks.Descriptor{Family: "viz", TypeName: "point"}
}

func (p *Point[T]) OnAttach(ctx facet.AttachContext) {
	p.Core.OnAttach()
	p.cleanups = append(p.cleanups,
		p.Store.OnInsertSubscribe(func(e store.CollectionInsertEvent[T]) {
			p.Invalidate(facet.DirtyProjection)
		}),
		p.Store.OnRemoveSubscribe(func(e store.CollectionRemoveEvent[T]) {
			p.Invalidate(facet.DirtyProjection)
		}),
		p.Store.OnUpdateSubscribe(func(e store.CollectionUpdateEvent[T]) {
			p.Invalidate(facet.DirtyProjection)
		}),
		p.Store.OnReplaceSubscribe(func(signal.Unit) {
			p.Invalidate(facet.DirtyProjection)
		}),
	)
}

func (p *Point[T]) OnDetach() {
	p.Core.OnDetach()
	for _, c := range p.cleanups {
		if c != nil {
			c()
		}
	}
	p.cleanups = nil
}

func (p *Point[T]) OnActivate()   { p.Core.OnActivate() }
func (p *Point[T]) OnDeactivate() { p.Core.OnDeactivate() }

func (p *Point[T]) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	return p.DefaultAnchors(p.Layout.ArrangedBounds, ctx)
}

func (p *Point[T]) buildCommands(bounds gfx.Rect) []gfx.Command {
	if bounds.IsEmpty() || p.Store == nil || p.XScale == nil || p.YScale == nil {
		return nil
	}
	xs := p.XScale.Get()
	ys := p.YScale.Get()
	items := p.Store.All()

	pts := make([]gfx.Point, 0, len(items))
	for _, item := range items {
		px := xs.Map(p.X(item))
		py := ys.Map(p.Y(item))
		pts = append(pts, gfx.Point{
			X: bounds.Min.X + float32(px),
			Y: bounds.Min.Y + float32(py),
		})
	}
	if len(pts) == 0 {
		return nil
	}
	return []gfx.Command{
		gfx.DrawPoints{
			Points: pts,
			Radius: p.Radius.Get(),
			Brush:  gfx.SolidBrush(p.Color),
		},
	}
}
