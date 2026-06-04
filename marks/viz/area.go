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

// Area renders an ordered data series as a filled area from the data line
// down to a baseline (typically y=0 or the bottom of the scale range).
type Area[T any] struct {
	marks.Core

	Store    *store.CollectionStore[T]
	X, Y     func(T) float64
	XScale   *reactive.ReactiveScale
	YScale   *reactive.ReactiveScale
	Color    gfx.Color
	Baseline marks.Binding[float64]

	cleanups []func()
}

var _ facet.FacetImpl = (*Area[int])(nil)
var _ layout.AnchorExporter = (*Area[int])(nil)
var _ marks.Mark = (*Area[int])(nil)

// NewArea constructs an area series mark.
func NewArea[T any](
	store *store.CollectionStore[T],
	x, y func(T) float64,
	xScale, yScale *reactive.ReactiveScale,
) *Area[T] {
	a := &Area[T]{
		Store:    store,
		X:        x,
		Y:        y,
		XScale:   xScale,
		YScale:   yScale,
		Color:    gfx.Color{R: 0.2, G: 0.4, B: 0.8, A: 0.3},
		Baseline: marks.Const(0.0),
	}
	a.Core.Facet = facet.NewFacet()
	a.AddBinding(a.Baseline)

	a.Layout.OnMeasure = func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
		return facet.MeasureResult{Size: constraints.MaxSize}
	}
	a.Layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		a.Layout.ArrangedBounds = bounds
	}
	a.BuildCommands = func(ctx facet.ProjectionContext) []gfx.Command {
		return a.buildCommands(a.Layout.ArrangedBounds)
	}
	a.RegisterRoles()
	return a
}

func (a *Area[T]) Base() *facet.Facet {
	a.Facet.BindImpl(a)
	return &a.Facet
}

func (a *Area[T]) Descriptor() marks.Descriptor {
	return marks.Descriptor{Family: "viz", TypeName: "area"}
}

func (a *Area[T]) OnAttach(ctx facet.AttachContext) {
	a.Core.OnAttach()
	a.cleanups = append(a.cleanups,
		a.Store.OnInsertSubscribe(func(e store.CollectionInsertEvent[T]) {
			a.Invalidate(facet.DirtyProjection)
		}),
		a.Store.OnRemoveSubscribe(func(e store.CollectionRemoveEvent[T]) {
			a.Invalidate(facet.DirtyProjection)
		}),
		a.Store.OnUpdateSubscribe(func(e store.CollectionUpdateEvent[T]) {
			a.Invalidate(facet.DirtyProjection)
		}),
		a.Store.OnReplaceSubscribe(func(signal.Unit) {
			a.Invalidate(facet.DirtyProjection)
		}),
	)
}

func (a *Area[T]) OnDetach() {
	a.Core.OnDetach()
	for _, c := range a.cleanups {
		if c != nil {
			c()
		}
	}
	a.cleanups = nil
}

func (a *Area[T]) OnActivate()   { a.Core.OnActivate() }
func (a *Area[T]) OnDeactivate() { a.Core.OnDeactivate() }

func (a *Area[T]) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	return a.DefaultAnchors(a.Layout.ArrangedBounds, ctx)
}

func (a *Area[T]) buildCommands(bounds gfx.Rect) []gfx.Command {
	if bounds.IsEmpty() || a.Store == nil || a.XScale == nil || a.YScale == nil {
		return nil
	}
	xs := a.XScale.Get()
	ys := a.YScale.Get()
	items := a.Store.All()
	if len(items) == 0 {
		return nil
	}

	segments := make([]gfx.PathSegment, 0, len(items)*2+2)
	baselineY := float32(ys.Map(a.Baseline.Get()))

	// Forward: data points forming the top edge
	for i, item := range items {
		verb := gfx.PathLineTo
		if i == 0 {
			verb = gfx.PathMoveTo
		}
		segments = append(segments, gfx.PathSegment{
			Verb: verb,
			Pts: [3]gfx.Point{{
				X: bounds.Min.X + float32(xs.Map(a.X(item))),
				Y: bounds.Min.Y + float32(ys.Map(a.Y(item))),
			}},
		})
	}

	// Return along the baseline in reverse order
	for i := len(items) - 1; i >= 0; i-- {
		x := bounds.Min.X + float32(xs.Map(a.X(items[i])))
		segments = append(segments, gfx.PathSegment{
			Verb: gfx.PathLineTo,
			Pts:  [3]gfx.Point{{X: x, Y: bounds.Min.Y + baselineY}},
		})
	}

	segments = append(segments, gfx.PathSegment{Verb: gfx.PathClose})

	return []gfx.Command{
		gfx.FillPath{
			Path:  gfx.Path{Segments: segments},
			Brush: gfx.SolidBrush(a.Color),
		},
	}
}
