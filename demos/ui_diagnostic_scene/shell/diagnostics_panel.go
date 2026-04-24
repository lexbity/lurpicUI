package shell

import (
	"fmt"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
	diag "codeburg.org/lexbit/ui_diagnostic_scene/diagnostics"
)

// DiagnosticsPanelFacet displays diagnostic information and overlays
type DiagnosticsPanelFacet struct {
	facet.Facet
	layout  facet.LayoutRole
	render  facet.RenderRole
	theme   theme.Context
	shaper  *text.Shaper
	adapter *diag.Adapter

	// Panel state
	overlayEnabled bool
	showBounds     bool
	showHitRegions bool
	showFocus      bool
}

// NewDiagnosticsPanelFacet constructs the diagnostics panel
func NewDiagnosticsPanelFacet(th theme.Context, shaper *text.Shaper, adapter *diag.Adapter) *DiagnosticsPanelFacet {
	d := &DiagnosticsPanelFacet{
		Facet:   facet.NewFacet(),
		theme:   th,
		shaper:  shaper,
		adapter: adapter,
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

	y += 8
	y = d.renderSummaryLine(list, bounds, y, d.renderSceneSummary())
	y = d.renderSummaryLine(list, bounds, y, d.renderOverlaySummary())
	y = d.renderSummaryLine(list, bounds, y, d.renderFocusSummary())
	y = d.renderSummaryLine(list, bounds, y, d.renderInvalidationSummary())
	y = d.renderSummaryLine(list, bounds, y, d.renderHitSummary())
	y = d.renderSummaryLine(list, bounds, y, d.renderRenderSummary())
	y = d.renderSummaryLine(list, bounds, y, d.renderAnchorSummary())
	y = d.renderSummaryLine(list, bounds, y, d.renderFrameSummary())
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

func (d *DiagnosticsPanelFacet) renderSummaryLine(list *gfx.CommandList, bounds gfx.Rect, y float32, lineText string) float32 {
	if lineText == "" {
		return y
	}
	style := d.theme.TextStyle(theme.TextBodyS)
	layout := d.shaper.ShapeSimple(lineText, style)
	if layout != nil && len(layout.Lines) > 0 {
		line := layout.Lines[0]
		origin := gfx.Point{X: bounds.Min.X + 12, Y: y + line.Baseline}
		for _, run := range line.Runs {
			list.Add(gfx.DrawGlyphRun{
				Run:    run,
				Origin: origin,
				Brush:  gfx.SolidBrush(d.theme.Color(theme.ColorText)),
			})
		}
		return y + layout.Bounds.Height() + 4
	}
	return y + 18
}

// SetOverlayEnabled toggles the overlay
func (d *DiagnosticsPanelFacet) SetOverlayEnabled(enabled bool) {
	d.overlayEnabled = enabled
	d.syncOverlayState()
	d.Invalidate(facet.DirtyProjection)
}

// SetShowBounds toggles bounds visualization
func (d *DiagnosticsPanelFacet) SetShowBounds(show bool) {
	d.showBounds = show
	d.syncOverlayState()
	d.Invalidate(facet.DirtyProjection)
}

// SetShowHitRegions toggles hit region visualization
func (d *DiagnosticsPanelFacet) SetShowHitRegions(show bool) {
	d.showHitRegions = show
	d.syncOverlayState()
	d.Invalidate(facet.DirtyProjection)
}

// SetShowFocus toggles focus chain visualization
func (d *DiagnosticsPanelFacet) SetShowFocus(show bool) {
	d.showFocus = show
	d.syncOverlayState()
	d.Invalidate(facet.DirtyProjection)
}

func (d *DiagnosticsPanelFacet) syncOverlayState() {
	if d == nil || d.adapter == nil {
		return
	}
	overlays := diag.NewActiveOverlays(d.adapter.GetSceneSummary().SceneID)
	if d.overlayEnabled {
		overlays.SetEnabled(diag.OverlayAll, true)
	}
	overlays.SetEnabled(diag.OverlayBounds, d.showBounds)
	overlays.SetEnabled(diag.OverlayHitRegions, d.showHitRegions)
	overlays.SetEnabled(diag.OverlayFocus, d.showFocus)
	d.adapter.SetActiveOverlays(overlays)
}

func (d *DiagnosticsPanelFacet) renderSceneSummary() string {
	if d == nil || d.adapter == nil {
		return ""
	}
	scene := d.adapter.GetSceneSummary()
	if scene.SceneID == "" && scene.SceneName == "" {
		return ""
	}
	caps := make([]string, 0, 4)
	if scene.SupportsScreenshot {
		caps = append(caps, "shot")
	}
	if scene.SupportsSnapshot {
		caps = append(caps, "snap")
	}
	if scene.SupportsThemeSwitch {
		caps = append(caps, "theme")
	}
	if scene.SupportsDensity {
		caps = append(caps, "density")
	}
	return fmt.Sprintf("Scene: %s (%s) families=%v caps=%v", scene.SceneID, scene.SceneName, scene.Families, caps)
}

func (d *DiagnosticsPanelFacet) renderOverlaySummary() string {
	if d == nil || d.adapter == nil {
		return ""
	}
	overlays := d.adapter.GetActiveOverlays()
	return fmt.Sprintf("Overlays: enabled=%t active=%v", overlays.AnyEnabled(), overlays.EnabledList())
}

func (d *DiagnosticsPanelFacet) renderFocusSummary() string {
	if d == nil || d.adapter == nil {
		return ""
	}
	focus := d.adapter.GetFocusSummary()
	if !focus.HasFocus && len(focus.TabOrder) == 0 {
		return "Focus: none"
	}
	return fmt.Sprintf("Focus: owner=%d type=%s depth=%d tab=%d", focus.ActiveFocusOwner, focus.FocusOwnerType, focus.FocusDepth(), len(focus.TabOrder))
}

func (d *DiagnosticsPanelFacet) renderInvalidationSummary() string {
	if d == nil || d.adapter == nil {
		return ""
	}
	inv := d.adapter.GetInvalidationSummary()
	return fmt.Sprintf("Invalidation: dirty=%d flags=%v", inv.TotalDirtyFacets, inv.ByFlag)
}

func (d *DiagnosticsPanelFacet) renderHitSummary() string {
	if d == nil || d.adapter == nil {
		return ""
	}
	hit := d.adapter.GetHitSummary()
	if hit.IsEmpty() {
		return "Hit: none"
	}
	return fmt.Sprintf("Hit: regions=%d types=%v", hit.TotalRegions, hit.RegionsByType)
}

func (d *DiagnosticsPanelFacet) renderRenderSummary() string {
	if d == nil || d.adapter == nil {
		return ""
	}
	render := d.adapter.GetRenderBatchSummary()
	if render.IsEmpty() {
		return "Render: none"
	}
	return fmt.Sprintf("Render: batches=%d layers=%v commands=%d", render.TotalBatches, render.BatchesByLayer, render.TotalCommands)
}

func (d *DiagnosticsPanelFacet) renderAnchorSummary() string {
	if d == nil || d.adapter == nil {
		return ""
	}
	anchors := d.adapter.GetAnchorSummary()
	if anchors.IsEmpty() {
		return "Anchors: none"
	}
	return fmt.Sprintf("Anchors: total=%d parents=%d", anchors.TotalAnchors, len(anchors.ByParent))
}

func (d *DiagnosticsPanelFacet) renderFrameSummary() string {
	if d == nil || d.adapter == nil {
		return ""
	}
	stats := d.adapter.GetFrameStats()
	if stats == nil {
		return ""
	}
	latest := stats.Latest()
	if latest.FrameNumber == 0 && latest.TotalDuration == 0 {
		return "Frame: none"
	}
	return fmt.Sprintf("Frame: #%d fps=%.1f dirty=%d projected=%d batches=%d", latest.FrameNumber, latest.FPS(), latest.DirtyFacetCount, latest.ProjectedFacetCount, latest.RenderBatchCount)
}
