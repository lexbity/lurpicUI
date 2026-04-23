package shell

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
)

// DiagnosticsPanelFacet displays diagnostic information and overlays
type DiagnosticsPanelFacet struct {
	facet.Facet
	layout facet.LayoutRole
	render facet.RenderRole
	theme  theme.Context
	shaper *text.Shaper

	// Panel state
	overlayEnabled bool
	showBounds     bool
	showHitRegions bool
	showFocus      bool
}

// NewDiagnosticsPanelFacet constructs the diagnostics panel
func NewDiagnosticsPanelFacet(th theme.Context, shaper *text.Shaper) *DiagnosticsPanelFacet {
	d := &DiagnosticsPanelFacet{
		Facet:  facet.NewFacet(),
		theme:  th,
		shaper: shaper,
	}

	d.layout.OnMeasure = func(c facet.Constraints) gfx.Size {
		return gfx.Size{W: c.MaxSize.W, H: c.MaxSize.H}
	}
	d.layout.OnArrange = func(bounds gfx.Rect) {
		d.layout.ArrangedBounds = bounds
	}
	d.AddRole(&d.layout)

	d.render.OnCollect = func(list *gfx.CommandList, bounds gfx.Rect) {
		d.renderPanel(list, bounds)
	}
	d.AddRole(&d.render)

	return d
}

func (d *DiagnosticsPanelFacet) Base() *facet.Facet {
	d.Facet.BindImpl(d)
	return &d.Facet
}

func (d *DiagnosticsPanelFacet) OnAttach(ctx facet.AttachContext) {}
func (d *DiagnosticsPanelFacet) OnDetach()                        {}
func (d *DiagnosticsPanelFacet) OnActivate()                      {}
func (d *DiagnosticsPanelFacet) OnDeactivate()                    {}

func (d *DiagnosticsPanelFacet) renderPanel(list *gfx.CommandList, bounds gfx.Rect) {
	if list == nil || bounds.IsEmpty() {
		return
	}

	// Background
	list.Add(gfx.FillRect{
		Rect:  bounds,
		Brush: gfx.SolidBrush(d.theme.Color(theme.ColorSurface)),
	})

	// Left border
	list.Add(gfx.FillRect{
		Rect:  gfx.RectFromXYWH(bounds.Min.X, bounds.Min.Y, 1, bounds.Height()),
		Brush: gfx.SolidBrush(d.theme.Color(theme.ColorBorder)),
	})

	if d.shaper == nil {
		return
	}

	// Header
	y := bounds.Min.Y + 12
	y = d.renderHeader(list, bounds, y)

	// Status items
	y += 8
	y = d.renderStatusItem(list, bounds, y, "Overlays", d.overlayEnabled)
	y = d.renderStatusItem(list, bounds, y, "Bounds", d.showBounds)
	y = d.renderStatusItem(list, bounds, y, "Hit Regions", d.showHitRegions)
	y = d.renderStatusItem(list, bounds, y, "Focus Chain", d.showFocus)

	// Empty state message
	y += 16
	d.renderEmptyState(list, bounds, y)
}

func (d *DiagnosticsPanelFacet) renderHeader(list *gfx.CommandList, bounds gfx.Rect, y float32) float32 {
	text := "Diagnostics"
	style := d.theme.TextStyle(theme.TextLabelS)
	layout := d.shaper.ShapeSimple(text, style)
	if layout != nil && len(layout.Lines) > 0 {
		line := layout.Lines[0]
		origin := gfx.Point{X: bounds.Min.X + 12, Y: y + line.Baseline}
		for _, run := range line.Runs {
			list.Add(gfx.DrawGlyphRun{
				Run:    run,
				Origin: origin,
				Brush:  gfx.SolidBrush(d.theme.Color(theme.ColorTextSecondary)),
			})
		}
		return y + layout.Bounds.Height() + 8
	}
	return y + 20
}

func (d *DiagnosticsPanelFacet) renderStatusItem(list *gfx.CommandList, bounds gfx.Rect, y float32, label string, enabled bool) float32 {
	// Checkbox
	checkChar := "☐"
	if enabled {
		checkChar = "☑"
	}

	checkStyle := d.theme.TextStyle(theme.TextBodyS)
	checkLayout := d.shaper.ShapeSimple(checkChar, checkStyle)
	if checkLayout != nil && len(checkLayout.Lines) > 0 {
		line := checkLayout.Lines[0]
		origin := gfx.Point{X: bounds.Min.X + 12, Y: y + 16}
		for _, run := range line.Runs {
			list.Add(gfx.DrawGlyphRun{
				Run:    run,
				Origin: origin,
				Brush:  gfx.SolidBrush(d.theme.Color(theme.ColorText)),
			})
		}
	}

	// Label
	labelStyle := d.theme.TextStyle(theme.TextBodyS)
	labelLayout := d.shaper.ShapeSimple(label, labelStyle)
	if labelLayout != nil && len(labelLayout.Lines) > 0 {
		line := labelLayout.Lines[0]
		origin := gfx.Point{X: bounds.Min.X + 36, Y: y + 16}
		for _, run := range line.Runs {
			list.Add(gfx.DrawGlyphRun{
				Run:    run,
				Origin: origin,
				Brush:  gfx.SolidBrush(d.theme.Color(theme.ColorText)),
			})
		}
	}

	return y + 24
}

func (d *DiagnosticsPanelFacet) renderEmptyState(list *gfx.CommandList, bounds gfx.Rect, y float32) {
	text := "(empty - Phase 2)"
	style := d.theme.TextStyle(theme.TextBodyS)
	layout := d.shaper.ShapeSimple(text, style)
	if layout != nil && len(layout.Lines) > 0 {
		line := layout.Lines[0]
		origin := gfx.Point{X: bounds.Min.X + 12, Y: y + line.Baseline}
		for _, run := range line.Runs {
			list.Add(gfx.DrawGlyphRun{
				Run:    run,
				Origin: origin,
				Brush:  gfx.SolidBrush(d.theme.Color(theme.ColorTextDisabled)),
			})
		}
	}
}

// SetOverlayEnabled toggles the overlay
func (d *DiagnosticsPanelFacet) SetOverlayEnabled(enabled bool) {
	d.overlayEnabled = enabled
	d.Invalidate(facet.DirtyProjection)
}

// SetShowBounds toggles bounds visualization
func (d *DiagnosticsPanelFacet) SetShowBounds(show bool) {
	d.showBounds = show
	d.Invalidate(facet.DirtyProjection)
}

// SetShowHitRegions toggles hit region visualization
func (d *DiagnosticsPanelFacet) SetShowHitRegions(show bool) {
	d.showHitRegions = show
	d.Invalidate(facet.DirtyProjection)
}

// SetShowFocus toggles focus chain visualization
func (d *DiagnosticsPanelFacet) SetShowFocus(show bool) {
	d.showFocus = show
	d.Invalidate(facet.DirtyProjection)
}
