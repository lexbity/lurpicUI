package viz

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/scale"
	"codeburg.org/lexbit/lurpicui/scale/reactive"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/store"
)

// Bar renders a bar chart from categorized data using a band scale (x) and
// a linear scale (y). Supports InvertRange for hit-testing.
type Bar[T any] struct {
	marks.Core

	Store    *store.CollectionStore[T]
	Cat      func(T) string
	Value    func(T) float64
	YScale   *reactive.ReactiveScale
	Padding  marks.Binding[float32]
	Color    gfx.Color
	Baseline marks.Binding[float64]

	bandMembers []string
	bandScale   scale.BandScale
	barRects    []gfx.Rect
	hitDirty    bool

	cleanups []func()
}

var _ facet.FacetImpl = (*Bar[int])(nil)
var _ layout.AnchorExporter = (*Bar[int])(nil)
var _ marks.Mark = (*Bar[int])(nil)

// NewBar constructs a bar mark.
func NewBar[T any](
	store *store.CollectionStore[T],
	cat func(T) string,
	value func(T) float64,
	yScale *reactive.ReactiveScale,
) *Bar[T] {
	b := &Bar[T]{
		Store:    store,
		Cat:      cat,
		Value:    value,
		YScale:   yScale,
		Padding:  marks.Const[float32](0.1),
		Color:    gfx.Color{R: 0.2, G: 0.4, B: 0.8, A: 1},
		Baseline: marks.Const(0.0),
		hitDirty: true,
	}
	b.Facet = facet.NewFacet()
	b.AddBinding(b.Padding)
	b.AddBinding(b.Baseline)

	b.Layout.OnMeasure = func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
		return facet.MeasureResult{Size: constraints.MaxSize}
	}
	b.Layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		b.Layout.ArrangedBounds = bounds
	}
	b.Hit.OnHitTest = func(p gfx.Point) facet.HitResult {
		return b.hitTest(p)
	}
	b.BuildCommands = func(ctx facet.ProjectionContext) []gfx.Command {
		return b.buildCommands(b.Layout.ArrangedBounds)
	}
	b.RegisterRoles()
	return b
}

func (b *Bar[T]) Base() *facet.Facet {
	b.BindImpl(b)
	return &b.Facet
}

func (b *Bar[T]) Descriptor() marks.Descriptor {
	return marks.Descriptor{Family: "viz", TypeName: "bar"}
}

func (b *Bar[T]) OnAttach(ctx facet.AttachContext) {
	b.Core.OnAttach()
	b.cleanups = append(b.cleanups,
		b.Store.OnInsertSubscribe(func(e store.CollectionInsertEvent[T]) {
			b.hitDirty = true
			b.Invalidate(facet.DirtyProjection | facet.DirtyHit)
		}),
		b.Store.OnRemoveSubscribe(func(e store.CollectionRemoveEvent[T]) {
			b.hitDirty = true
			b.Invalidate(facet.DirtyProjection | facet.DirtyHit)
		}),
		b.Store.OnUpdateSubscribe(func(e store.CollectionUpdateEvent[T]) {
			b.hitDirty = true
			b.Invalidate(facet.DirtyProjection | facet.DirtyHit)
		}),
		b.Store.OnReplaceSubscribe(func(signal.Unit) {
			b.hitDirty = true
			b.Invalidate(facet.DirtyProjection | facet.DirtyHit)
		}),
	)
}

func (b *Bar[T]) OnDetach() {
	b.Core.OnDetach()
	for _, c := range b.cleanups {
		if c != nil {
			c()
		}
	}
	b.cleanups = nil
}

func (b *Bar[T]) OnActivate()   { b.Core.OnActivate() }
func (b *Bar[T]) OnDeactivate() { b.Core.OnDeactivate() }

func (b *Bar[T]) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	return b.DefaultAnchors(b.Layout.ArrangedBounds, ctx)
}

func (b *Bar[T]) buildCommands(bounds gfx.Rect) []gfx.Command {
	if bounds.IsEmpty() || b.Store == nil || b.YScale == nil {
		return nil
	}
	items := b.Store.All()
	if len(items) == 0 {
		return nil
	}

	// Build band scale from data categories
	b.bandMembers = make([]string, len(items))
	for i, item := range items {
		b.bandMembers[i] = b.Cat(item)
	}
	b.bandScale = scale.NewBand(b.bandMembers,
		scale.WithPaddingInner(float64(b.Padding.Get())),
		scale.WithRange(0, float64(bounds.Width())),
	)

	ys := b.YScale.Get()
	baselinePixel := ys.Map(b.Baseline.Get())

	cmds := make([]gfx.Command, 0, len(items))
	b.barRects = make([]gfx.Rect, len(items))
	b.hitDirty = false

	for i, item := range items {
		start, width, ok := b.bandScale.Band(b.Cat(item))
		if !ok {
			continue
		}
		val := ys.Map(b.Value(item))
		x := bounds.Min.X + float32(start)
		w := float32(width)

		var rect gfx.Rect
		if val >= baselinePixel {
			rect = gfx.RectFromXYWH(x, bounds.Min.Y+float32(baselinePixel), w, float32(val-baselinePixel))
		} else {
			rect = gfx.RectFromXYWH(x, bounds.Min.Y+float32(val), w, float32(baselinePixel-val))
		}

		b.barRects[i] = rect
		cmds = append(cmds, gfx.FillRect{
			Rect:  rect,
			Brush: gfx.SolidBrush(b.Color),
		})
	}
	return cmds
}

// HitMember returns the category whose bar contains the given local point.
// Returns ("", false) if no bar is hit.
func (b *Bar[T]) HitMember(p gfx.Point) (string, bool) {
	if b.hitDirty || len(b.barRects) == 0 {
		return "", false
	}
	for i, rect := range b.barRects {
		if rect.Contains(p) && i < len(b.bandMembers) {
			return b.bandMembers[i], true
		}
	}
	// Also try band scale InvertRange
	if b.bandScale.Bandwidth() > 0 {
		return b.bandScale.InvertRange(float64(p.X - b.Layout.ArrangedBounds.Min.X))
	}
	return "", false
}

func (b *Bar[T]) hitTest(p gfx.Point) facet.HitResult {
	_, ok := b.HitMember(p)
	if !ok {
		return facet.HitResult{}
	}
	return facet.HitResult{Hit: true, MarkID: 1}
}
