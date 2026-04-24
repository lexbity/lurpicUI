package ui

import (
	"fmt"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/ui_catalog/model"
	"codeburg.org/lexbit/ui_catalog/store"
)

// HeaderFacet displays app title and build metadata.
type HeaderFacet struct {
	facet.Facet
	layout   facet.LayoutRole
	render   facet.RenderRole
	hit      facet.HitRole
	th       theme.Context
	shaper   *text.Shaper
	meta     model.BuildMetadata
	themeSub signal.SubscriptionID
}

// NewHeaderFacet creates a new header facet.
func NewHeaderFacet(th theme.Context, shaper *text.Shaper, meta model.BuildMetadata) *HeaderFacet {
	h := &HeaderFacet{
		Facet:  facet.NewFacet(),
		th:     th,
		shaper: shaper,
		meta:   meta,
	}

	h.layout.OnMeasure = func(c facet.Constraints) gfx.Size {
		return gfx.Size{W: c.MaxSize.W, H: headerHeight}
	}

	h.layout.OnArrange = func(bounds gfx.Rect) {
		h.layout.ArrangedBounds = bounds
	}
	h.AddRole(&h.layout)

	h.render.OnCollect = func(list *gfx.CommandList, bounds gfx.Rect) {
		h.renderHeader(list, bounds)
	}
	h.AddRole(&h.render)

	// Hit role for interactive elements
	h.hit.OnHit = func(ev facet.HitEvent) bool {
		return h.handleHit(ev)
	}
	h.AddRole(&h.hit)

	return h
}

// Base returns the base facet.
func (f *HeaderFacet) Base() *facet.Facet {
	f.Facet.BindImpl(f)
	return &f.Facet
}

// OnAttach handles attachment.
func (f *HeaderFacet) OnAttach(ctx facet.AttachContext) {
	// Subscribe to theme changes to re-render
	f.themeSub = store.ThemeStore.OnChange.Subscribe(func(change signal.Change[store.ThemeMode]) {
		f.Facet.Invalidate(facet.DirtyProjection)
	})
}

// OnDetach handles detachment.
func (f *HeaderFacet) OnDetach() {
	store.ThemeStore.OnChange.Unsubscribe(f.themeSub)
}

// OnActivate handles activation.
func (f *HeaderFacet) OnActivate() {}

// OnDeactivate handles deactivation.
func (f *HeaderFacet) OnDeactivate() {}

func (f *HeaderFacet) renderHeader(list *gfx.CommandList, bounds gfx.Rect) {
	if list == nil || bounds.IsEmpty() {
		return
	}

	// Background
	list.Add(gfx.FillRect{
		Rect:  bounds,
		Brush: gfx.SolidBrush(f.th.Color(theme.ColorSurface)),
	})

	// Bottom border
	borderBrush := f.th.Color(theme.ColorBorder)
	list.Add(gfx.FillRect{
		Rect:  gfx.RectFromXYWH(bounds.Min.X, bounds.Max.Y-1, bounds.Width(), 1),
		Brush: gfx.SolidBrush(borderBrush),
	})

	if f.shaper == nil {
		return
	}

	// Title text
	title := "UI Catalog"
	f.drawText(list, 16, bounds, title, theme.TextHeadingS, f.th.Color(theme.ColorPrimary))

	// Theme and Density selectors in the center-right
	rightX := bounds.Max.X - 16

	// Density button
	densityBtn := store.GetDensity().String()
	densityStyle := f.th.TextStyle(theme.TextBodyS)
	densityLayout := f.shaper.ShapeSimple(densityBtn, densityStyle)
	if densityLayout != nil && len(densityLayout.Lines) > 0 {
		line := densityLayout.Lines[0]
		densityWidth := line.Bounds.Width() + 24 // padding
		rightX -= densityWidth
		f.drawButton(list, rightX, bounds.Min.Y+(bounds.Height()-24)/2, densityWidth, 24, densityBtn, theme.TextBodyS)
		rightX -= 8 // gap
	}

	// Theme button
	themeBtn := store.GetTheme().String()
	themeStyle := f.th.TextStyle(theme.TextBodyS)
	themeLayout := f.shaper.ShapeSimple(themeBtn, themeStyle)
	if themeLayout != nil && len(themeLayout.Lines) > 0 {
		line := themeLayout.Lines[0]
		themeWidth := line.Bounds.Width() + 24 // padding
		rightX -= themeWidth
		f.drawButton(list, rightX, bounds.Min.Y+(bounds.Height()-24)/2, themeWidth, 24, themeBtn, theme.TextBodyS)
		rightX -= 8 // gap
	}

	// Version info on the right
	versionText := fmt.Sprintf("v%s | %s | %s", f.meta.Version, f.meta.Backend, f.meta.ThemeEngine)
	versionStyle := f.th.TextStyle(theme.TextBodyS)
	versionLayout := f.shaper.ShapeSimple(versionText, versionStyle)
	if versionLayout != nil && len(versionLayout.Lines) > 0 {
		line := versionLayout.Lines[0]
		width := line.Bounds.Width()
		x := rightX - width
		y := bounds.Min.Y + (bounds.Height()-versionLayout.Bounds.Height())/2
		f.drawTextLine(list, x, y, line, f.th.Color(theme.ColorTextSecondary))
	}
}

// drawButton draws a simple button-like element.
func (f *HeaderFacet) drawButton(list *gfx.CommandList, x, y, w, h float32, label string, textToken theme.TextToken) {
	// Button background
	list.Add(gfx.FillRect{
		Rect:  gfx.RectFromXYWH(x, y, w, h),
		Brush: gfx.SolidBrush(f.th.Color(theme.ColorSurfaceVariant)),
	})
	// Button border
	borderColor := f.th.Color(theme.ColorBorder)
	list.Add(gfx.FillRect{
		Rect:  gfx.RectFromXYWH(x, y, w, 1),
		Brush: gfx.SolidBrush(borderColor),
	})
	list.Add(gfx.FillRect{
		Rect:  gfx.RectFromXYWH(x, y+h-1, w, 1),
		Brush: gfx.SolidBrush(borderColor),
	})
	list.Add(gfx.FillRect{
		Rect:  gfx.RectFromXYWH(x, y, 1, h),
		Brush: gfx.SolidBrush(borderColor),
	})
	list.Add(gfx.FillRect{
		Rect:  gfx.RectFromXYWH(x+w-1, y, 1, h),
		Brush: gfx.SolidBrush(borderColor),
	})
	// Button text
	style := f.th.TextStyle(textToken)
	layout := f.shaper.ShapeSimple(label, style)
	if layout != nil && len(layout.Lines) > 0 {
		line := layout.Lines[0]
		textX := x + (w-line.Bounds.Width())/2
		textY := y + (h-layout.Bounds.Height())/2 + layout.Bounds.Height()/2
		f.drawTextLine(list, textX, textY, line, f.th.Color(theme.ColorText))
	}
}

func (f *HeaderFacet) drawText(list *gfx.CommandList, x float32, bounds gfx.Rect, s string, token theme.TextToken, color gfx.Color) {
	if list == nil || f.shaper == nil || s == "" {
		return
	}
	style := f.th.TextStyle(token)
	layout := f.shaper.ShapeSimple(s, style)
	if layout == nil || len(layout.Lines) == 0 {
		return
	}
	line := layout.Lines[0]
	y := bounds.Min.Y + (bounds.Height()-layout.Bounds.Height())/2 + layout.Bounds.Height() - (layout.Bounds.Max.Y - line.Baseline)
	f.drawTextLine(list, bounds.Min.X+x, y, line, color)
}

func (f *HeaderFacet) drawTextLine(list *gfx.CommandList, x, y float32, line text.ShapedLine, color gfx.Color) {
	origin := gfx.Point{X: x + line.Bounds.Min.X, Y: y + line.Baseline}
	for _, run := range line.Runs {
		list.Add(gfx.DrawGlyphRun{
			Run:    run,
			Origin: origin,
			Brush:  gfx.SolidBrush(color),
		})
	}
}
