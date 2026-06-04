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

// Line renders an ordered data series as a polyline.
type Line[T any] struct {
	marks.Core

	Store       *store.CollectionStore[T]
	X, Y        func(T) float64
	XScale      *reactive.ReactiveScale
	YScale      *reactive.ReactiveScale
	StrokeWidth marks.Binding[float32]
	Color       gfx.Color

	cleanups []func()
}

var _ facet.FacetImpl = (*Line[int])(nil)
var _ layout.AnchorExporter = (*Line[int])(nil)
var _ marks.Mark = (*Line[int])(nil)

// NewLine constructs a line series mark.
func NewLine[T any](
	store *store.CollectionStore[T],
	x, y func(T) float64,
	xScale, yScale *reactive.ReactiveScale,
) *Line[T] {
	l := &Line[T]{
		Store:       store,
		X:           x,
		Y:           y,
		XScale:      xScale,
		YScale:      yScale,
		StrokeWidth: marks.Const[float32](2),
		Color:       gfx.Color{R: 0.2, G: 0.4, B: 0.8, A: 1},
	}
	l.Core.Facet = facet.NewFacet()
	l.AddBinding(l.StrokeWidth)

	l.Layout.OnMeasure = func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
		return facet.MeasureResult{Size: constraints.MaxSize}
	}
	l.Layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		l.Layout.ArrangedBounds = bounds
	}
	l.BuildCommands = func(ctx facet.ProjectionContext) []gfx.Command {
		return l.buildCommands(l.Layout.ArrangedBounds)
	}
	l.RegisterRoles()
	return l
}

func (l *Line[T]) Base() *facet.Facet {
	l.Facet.BindImpl(l)
	return &l.Facet
}

func (l *Line[T]) Descriptor() marks.Descriptor {
	return marks.Descriptor{Family: "viz", TypeName: "line"}
}

func (l *Line[T]) OnAttach(ctx facet.AttachContext) {
	l.Core.OnAttach()
	l.subscribe()
}

func (l *Line[T]) OnDetach() {
	l.Core.OnDetach()
	for _, c := range l.cleanups {
		if c != nil {
			c()
		}
	}
	l.cleanups = nil
}

func (l *Line[T]) OnActivate()   { l.Core.OnActivate() }
func (l *Line[T]) OnDeactivate() { l.Core.OnDeactivate() }

func (l *Line[T]) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	return l.DefaultAnchors(l.Layout.ArrangedBounds, ctx)
}

func (l *Line[T]) subscribe() {
	l.cleanups = append(l.cleanups,
		l.Store.OnInsertSubscribe(func(e store.CollectionInsertEvent[T]) {
			l.Invalidate(facet.DirtyProjection)
		}),
		l.Store.OnRemoveSubscribe(func(e store.CollectionRemoveEvent[T]) {
			l.Invalidate(facet.DirtyProjection)
		}),
		l.Store.OnUpdateSubscribe(func(e store.CollectionUpdateEvent[T]) {
			l.Invalidate(facet.DirtyProjection)
		}),
		l.Store.OnReplaceSubscribe(func(signal.Unit) {
			l.Invalidate(facet.DirtyProjection)
		}),
	)
}

func (l *Line[T]) buildCommands(bounds gfx.Rect) []gfx.Command {
	if bounds.IsEmpty() || l.Store == nil || l.XScale == nil || l.YScale == nil {
		return nil
	}
	xs := l.XScale.Get()
	ys := l.YScale.Get()
	items := l.Store.All()
	if len(items) == 0 {
		return nil
	}

	pts := make([]gfx.Point, len(items))
	for i, item := range items {
		pts[i] = gfx.Point{
			X: bounds.Min.X + float32(xs.Map(l.X(item))),
			Y: bounds.Min.Y + float32(ys.Map(l.Y(item))),
		}
	}
	return []gfx.Command{
		gfx.DrawPolyline{
			Points: pts,
			Stroke: gfx.StrokeStyle{Width: l.StrokeWidth.Get()},
			Brush:  gfx.SolidBrush(l.Color),
		},
	}
}
