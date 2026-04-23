package ui

import (
	"fmt"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/ui_catalog/store"
)

// FooterFacet displays counts and status information.
type FooterFacet struct {
	facet.Facet
	layout       facet.LayoutRole
	render       facet.RenderRole
	th           theme.Context
	shaper       *text.Shaper
	subscription signal.SubscriptionID
}

// NewFooterFacet creates a new footer facet.
func NewFooterFacet(th theme.Context, shaper *text.Shaper) *FooterFacet {
	f := &FooterFacet{
		Facet:  facet.NewFacet(),
		th:     th,
		shaper: shaper,
	}

	f.layout.OnMeasure = func(c facet.Constraints) gfx.Size {
		return gfx.Size{W: c.MaxSize.W, H: footerHeight}
	}

	f.layout.OnArrange = func(bounds gfx.Rect) {
		f.layout.ArrangedBounds = bounds
	}
	f.AddRole(&f.layout)

	f.render.OnCollect = func(list *gfx.CommandList, bounds gfx.Rect) {
		f.renderFooter(list, bounds)
	}
	f.AddRole(&f.render)

	return f
}

// Base returns the base facet.
func (f *FooterFacet) Base() *facet.Facet {
	f.Facet.BindImpl(f)
	return &f.Facet
}

// OnAttach handles attachment.
func (f *FooterFacet) OnAttach(ctx facet.AttachContext) {
	f.subscription = store.FilterStore.OnChange.Subscribe(func(change signal.Change[store.FilterState]) {
		f.Invalidate(facet.DirtyProjection)
	})
}

// OnDetach handles detachment.
func (f *FooterFacet) OnDetach() {
	store.FilterStore.OnChange.Unsubscribe(f.subscription)
}

// OnActivate handles activation.
func (f *FooterFacet) OnActivate() {}

// OnDeactivate handles deactivation.
func (f *FooterFacet) OnDeactivate() {}

func (f *FooterFacet) renderFooter(list *gfx.CommandList, bounds gfx.Rect) {
	if list == nil || bounds.IsEmpty() {
		return
	}

	// Background
	list.Add(gfx.FillRect{
		Rect:  bounds,
		Brush: gfx.SolidBrush(f.th.Color(theme.ColorSurface)),
	})

	// Top border
	borderBrush := f.th.Color(theme.ColorBorder)
	list.Add(gfx.FillRect{
		Rect:  gfx.RectFromXYWH(bounds.Min.X, bounds.Min.Y, bounds.Width(), 1),
		Brush: gfx.SolidBrush(borderBrush),
	})

	if f.shaper == nil {
		return
	}

	inner := Inset(bounds, 8)
	if inner.IsEmpty() {
		return
	}

	counts := store.GetCounts()

	// Left side: counts
	leftText := fmt.Sprintf("%d / %d entries", counts.Filtered, counts.Total)
	leftStyle := f.th.TextStyle(theme.TextLabelS)
	leftLayout := f.shaper.ShapeSimple(leftText, leftStyle)
	if leftLayout != nil && len(leftLayout.Lines) > 0 {
		line := leftLayout.Lines[0]
		f.drawTextLine(list, inner.Min.X, inner.Min.Y, line, f.th.Color(theme.ColorTextSecondary))
	}

	// Right side: selection status
	rightText := "No selection"
	if sel, ok := store.SelectedEntry(store.CatalogInstance); ok {
		rightText = fmt.Sprintf("Selected: %s", sel.ID)
	}
	rightStyle := f.th.TextStyle(theme.TextLabelS)
	rightLayout := f.shaper.ShapeSimple(rightText, rightStyle)
	if rightLayout != nil && len(rightLayout.Lines) > 0 {
		line := rightLayout.Lines[0]
		x := inner.Max.X - line.Bounds.Width()
		f.drawTextLine(list, x, inner.Min.Y, line, f.th.Color(theme.ColorTextSecondary))
	}
}

func (f *FooterFacet) drawTextLine(list *gfx.CommandList, x, y float32, line text.ShapedLine, color gfx.Color) {
	origin := gfx.Point{X: x + line.Bounds.Min.X, Y: y + line.Baseline}
	for _, run := range line.Runs {
		list.Add(gfx.DrawGlyphRun{
			Run:    run,
			Origin: origin,
			Brush:  gfx.SolidBrush(color),
		})
	}
}
