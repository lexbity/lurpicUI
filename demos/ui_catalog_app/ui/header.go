package ui

import (
	"fmt"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/ui_catalog/model"
	"codeburg.org/lexbit/ui_catalog/store"
)

// HeaderFacet displays app title and build metadata.
type HeaderFacet struct {
	facet.Facet
	layout        facet.LayoutRole
	render        facet.RenderRole
	hit           facet.HitRole
	input         facet.InputRole
	th            theme.Context
	shaper        *text.Shaper
	meta          model.BuildMetadata
	themeSub      signal.SubscriptionID
	layoutProfile LayoutProfile
	compact       bool
	visible       []headerAction
	buttonRects   map[string]gfx.Rect
	activeButton  string

	OnToggleBrowse  signal.Signal[struct{}]
	OnToggleDetails signal.Signal[struct{}]
}

type headerAction struct {
	kind     string
	label    string
	rect     gfx.Rect
	activate func()
}

// NewHeaderFacet creates a new header facet.
func NewHeaderFacet(th theme.Context, shaper *text.Shaper, meta model.BuildMetadata) *HeaderFacet {
	h := &HeaderFacet{
		Facet:         facet.NewFacet(),
		th:            th,
		shaper:        shaper,
		meta:          meta,
		layoutProfile: DefaultLayoutProfile(),
		buttonRects:   make(map[string]gfx.Rect),
	}

	h.layout.OnMeasure = func(c facet.Constraints) gfx.Size {
		profile := h.layoutProfile
		if profile.HeaderHeight <= 0 {
			profile = DefaultLayoutProfile()
		}
		return gfx.Size{W: c.MaxSize.W, H: profile.HeaderHeight}
	}

	h.layout.OnArrange = func(bounds gfx.Rect) {
		h.layout.ArrangedBounds = bounds
		h.layoutControls(bounds)
	}
	h.AddRole(&h.layout)

	h.render.OnCollect = func(list *gfx.CommandList, bounds gfx.Rect) {
		h.renderHeader(list, bounds)
	}
	h.AddRole(&h.render)

	// Hit role for interactive elements
	h.hit.OnHitTest = func(p gfx.Point) facet.HitResult {
		if h.layout.ArrangedBounds.Contains(p) {
			return facet.HitResult{Hit: true}
		}
		return facet.HitResult{}
	}
	h.AddRole(&h.hit)

	h.input.OnPointer = func(e facet.PointerEvent) bool {
		if e.Kind != platform.PointerRelease || e.Button != platform.PointerLeft {
			return false
		}
		for _, control := range h.layoutControls(h.layout.ArrangedBounds) {
			if control.rect.Contains(e.Position) {
				if control.activate != nil {
					control.activate()
				}
				return true
			}
		}
		return false
	}
	h.AddRole(&h.input)

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

// SetLayoutProfile updates density-driven header geometry.
func (f *HeaderFacet) SetLayoutProfile(profile LayoutProfile) {
	if f == nil {
		return
	}
	f.layoutProfile = profile
	f.Invalidate(facet.DirtyLayout | facet.DirtyProjection)
}

type headerControl struct {
	kind     string
	rect     gfx.Rect
	activate func()
}

func (f *HeaderFacet) layoutControls(bounds gfx.Rect) []headerAction {
	if f == nil || bounds.IsEmpty() || f.shaper == nil {
		return nil
	}
	profile := f.layoutProfile
	if profile.HeaderInset <= 0 {
		profile = DefaultLayoutProfile()
	}
	inner := Inset(bounds, profile.HeaderInset)
	if inner.IsEmpty() {
		return nil
	}
	rightX := bounds.Max.X - profile.HeaderInset
	controls := make([]headerAction, 0, 4)
	f.compact = bounds.Width() < 900
	if f.buttonRects == nil {
		f.buttonRects = make(map[string]gfx.Rect)
	}
	for k := range f.buttonRects {
		delete(f.buttonRects, k)
	}

	addControl := func(kind, label string, activate func()) {
		style := f.th.TextStyle(theme.TextBodyS)
		layout := f.shaper.ShapeSimple(label, style)
		width := float32(72)
		if layout != nil && len(layout.Lines) > 0 {
			width = layout.Lines[0].Bounds.Width() + 24
		}
		rightX -= width
		rect := gfx.RectFromXYWH(rightX, bounds.Min.Y+(bounds.Height()-24)/2, width, 24)
		controls = append(controls, headerAction{kind: kind, label: label, rect: rect, activate: activate})
		f.buttonRects[kind] = rect
		rightX -= 8
	}

	if f.compact {
		addControl("browse", "Browse", func() { f.OnToggleBrowse.Emit(struct{}{}) })
		addControl("details", "Details", func() { f.OnToggleDetails.Emit(struct{}{}) })
		addControl("density", store.GetDensity().String(), func() {
			switch store.GetDensity() {
			case store.DensityNormal:
				store.SetDensity(store.DensityComfortable)
			case store.DensityComfortable:
				store.SetDensity(store.DensityCompact)
			default:
				store.SetDensity(store.DensityNormal)
			}
		})
		addControl("theme", store.GetTheme().String(), func() {
			switch store.GetTheme() {
			case store.ThemeSystem:
				store.SetTheme(store.ThemeLight)
			case store.ThemeLight:
				store.SetTheme(store.ThemeDark)
			default:
				store.SetTheme(store.ThemeSystem)
			}
		})
	} else {
		addControl("density", store.GetDensity().String(), func() {
			switch store.GetDensity() {
			case store.DensityNormal:
				store.SetDensity(store.DensityComfortable)
			case store.DensityComfortable:
				store.SetDensity(store.DensityCompact)
			default:
				store.SetDensity(store.DensityNormal)
			}
		})
		addControl("theme", store.GetTheme().String(), func() {
			switch store.GetTheme() {
			case store.ThemeSystem:
				store.SetTheme(store.ThemeLight)
			case store.ThemeLight:
				store.SetTheme(store.ThemeDark)
			default:
				store.SetTheme(store.ThemeSystem)
			}
		})
	}
	return controls
}

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

	if f.compact {
		for _, control := range f.layoutControls(bounds) {
			f.drawButton(list, control.rect.Min.X, control.rect.Min.Y, control.rect.Width(), control.rect.Height(), control.label, theme.TextBodyS)
		}
		return
	}

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
