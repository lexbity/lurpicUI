package ui

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/ui_catalog/model"
)

// Card constants for stable sizing
const (
	cardWidth  = 160
	cardHeight = 100
	cardMargin = 12
)

// CardFacet renders a single catalog entry card.
type CardFacet struct {
	facet.Facet
	layout   facet.LayoutRole
	render   facet.RenderRole
	hit      facet.HitRole
	input    facet.InputRole
	th       theme.Context
	shaper   *text.Shaper
	entry    *model.CatalogEntry
	bounds   gfx.Rect
	onClick  func()
	selected bool
	focused  bool
}

// SetSelected sets the selected state of the card.
func (f *CardFacet) SetSelected(selected bool) {
	if f.selected != selected {
		f.selected = selected
		f.Invalidate(facet.DirtyProjection)
	}
}

// SetFocused sets the focused state of the card.
func (f *CardFacet) SetFocused(focused bool) {
	if f.focused != focused {
		f.focused = focused
		f.Invalidate(facet.DirtyProjection)
	}
}

// NewCardFacet creates a new card facet for the given entry.
func NewCardFacet(th theme.Context, shaper *text.Shaper, entry *model.CatalogEntry) *CardFacet {
	c := &CardFacet{
		Facet:  facet.NewFacet(),
		th:     th,
		shaper: shaper,
		entry:  entry,
	}

	c.layout.OnMeasure = func(cons facet.Constraints) gfx.Size {
		return gfx.Size{W: cardWidth, H: cardHeight}
	}

	c.layout.OnArrange = func(bounds gfx.Rect) {
		c.bounds = bounds
		c.layout.ArrangedBounds = bounds
	}
	c.AddRole(&c.layout)

	c.render.OnCollect = func(list *gfx.CommandList, bounds gfx.Rect) {
		c.renderCard(list, bounds)
	}
	c.AddRole(&c.render)

	c.hit.OnHitTest = func(p gfx.Point) facet.HitResult {
		if c.bounds.Contains(p) {
			return facet.HitResult{Hit: true}
		}
		return facet.HitResult{}
	}
	c.AddRole(&c.hit)

	c.input.OnPointer = func(e facet.PointerEvent) bool {
		if e.Kind != platform.PointerRelease || e.Button != platform.PointerLeft {
			return false
		}
		if c.bounds.Contains(e.Position) && c.onClick != nil {
			c.onClick()
			return true
		}
		return false
	}
	c.AddRole(&c.input)

	return c
}

// Base returns the base facet.
func (f *CardFacet) Base() *facet.Facet {
	f.Facet.BindImpl(f)
	return &f.Facet
}

// OnAttach handles attachment.
func (f *CardFacet) OnAttach(ctx facet.AttachContext) {}

// OnDetach handles detachment.
func (f *CardFacet) OnDetach() {}

// OnActivate handles activation.
func (f *CardFacet) OnActivate() {}

// OnDeactivate handles deactivation.
func (f *CardFacet) OnDeactivate() {}

// SetOnClick sets the click handler.
func (f *CardFacet) SetOnClick(fn func()) {
	f.onClick = fn
}

// Entry returns the card's entry.
func (f *CardFacet) Entry() *model.CatalogEntry {
	return f.entry
}

func (f *CardFacet) renderCard(list *gfx.CommandList, bounds gfx.Rect) {
	if list == nil || bounds.IsEmpty() || f.entry == nil || f.shaper == nil {
		return
	}

	// Card background - highlight when selected
	bgColor := f.th.Color(theme.ColorSurface)
	if f.selected {
		bgColor = f.th.Color(theme.ColorSurfaceVariant)
	}
	list.Add(gfx.FillRect{
		Rect:  bounds,
		Brush: gfx.SolidBrush(bgColor),
	})

	// Border - thicker when selected or focused
	borderWidth := float32(1)
	if f.selected {
		borderWidth = 2
	}
	borderColor := f.th.Color(theme.ColorBorder)
	if f.selected {
		borderColor = f.th.Color(theme.ColorPrimary)
	} else if f.focused {
		borderColor = f.th.Color(theme.ColorBorderStrong)
	}

	// Top border
	list.Add(gfx.FillRect{
		Rect:  gfx.RectFromXYWH(bounds.Min.X, bounds.Min.Y, bounds.Width(), borderWidth),
		Brush: gfx.SolidBrush(borderColor),
	})
	// Bottom border
	list.Add(gfx.FillRect{
		Rect:  gfx.RectFromXYWH(bounds.Min.X, bounds.Max.Y-borderWidth, bounds.Width(), borderWidth),
		Brush: gfx.SolidBrush(borderColor),
	})
	// Left border
	list.Add(gfx.FillRect{
		Rect:  gfx.RectFromXYWH(bounds.Min.X, bounds.Min.Y, borderWidth, bounds.Height()),
		Brush: gfx.SolidBrush(borderColor),
	})
	// Right border
	list.Add(gfx.FillRect{
		Rect:  gfx.RectFromXYWH(bounds.Max.X-borderWidth, bounds.Min.Y, borderWidth, bounds.Height()),
		Brush: gfx.SolidBrush(borderColor),
	})

	// Focus ring (outside border)
	if f.focused {
		ringColor := f.th.Color(theme.ColorBorderStrong)
		ringWidth := float32(2)
		outerBounds := bounds.Inset(-ringWidth, -ringWidth)
		// Draw focus ring outline
		list.Add(gfx.FillRect{
			Rect:  gfx.RectFromXYWH(outerBounds.Min.X, outerBounds.Min.Y, outerBounds.Width(), ringWidth),
			Brush: gfx.SolidBrush(ringColor),
		})
		list.Add(gfx.FillRect{
			Rect:  gfx.RectFromXYWH(outerBounds.Min.X, outerBounds.Max.Y-ringWidth, outerBounds.Width(), ringWidth),
			Brush: gfx.SolidBrush(ringColor),
		})
		list.Add(gfx.FillRect{
			Rect:  gfx.RectFromXYWH(outerBounds.Min.X, outerBounds.Min.Y, ringWidth, outerBounds.Height()),
			Brush: gfx.SolidBrush(ringColor),
		})
		list.Add(gfx.FillRect{
			Rect:  gfx.RectFromXYWH(outerBounds.Max.X-ringWidth, outerBounds.Min.Y, ringWidth, outerBounds.Height()),
			Brush: gfx.SolidBrush(ringColor),
		})
	}

	inner := Inset(bounds, 8)
	if inner.IsEmpty() {
		return
	}

	// Entry ID (truncated if too long)
	y := inner.Min.Y
	idText := f.truncateText(f.entry.ID, inner.Width(), theme.TextBodyS)
	idStyle := f.th.TextStyle(theme.TextBodyS)
	idLayout := f.shaper.ShapeSimple(idText, idStyle)
	if idLayout != nil && len(idLayout.Lines) > 0 {
		line := idLayout.Lines[0]
		f.drawTextLine(list, inner.Min.X, y, line, f.th.Color(theme.ColorText))
		y += idLayout.Bounds.Height() + 4
	}

	// Display name (truncated)
	nameText := f.truncateText(f.entry.DisplayName, inner.Width(), theme.TextLabelS)
	nameStyle := f.th.TextStyle(theme.TextLabelS)
	nameLayout := f.shaper.ShapeSimple(nameText, nameStyle)
	if nameLayout != nil && len(nameLayout.Lines) > 0 {
		line := nameLayout.Lines[0]
		f.drawTextLine(list, inner.Min.X, y, line, f.th.Color(theme.ColorTextSecondary))
		y += nameLayout.Bounds.Height() + 8
	}

	// Coverage indicator at bottom
	covText := f.entry.Coverage.String()
	covStyle := f.th.TextStyle(theme.TextLabelS)
	covLayout := f.shaper.ShapeSimple(covText, covStyle)
	if covLayout != nil && len(covLayout.Lines) > 0 {
		line := covLayout.Lines[0]
		covColor := f.th.Color(theme.ColorTextSecondary)
		switch f.entry.Coverage {
		case model.CoverageImplemented:
			covColor = f.th.Color(theme.ColorSuccess)
		case model.CoveragePartial:
			covColor = f.th.Color(theme.ColorWarning)
		case model.CoveragePlaceholder, model.CoverageMissing:
			covColor = f.th.Color(theme.ColorError)
		}
		covY := inner.Max.Y - covLayout.Bounds.Height()
		f.drawTextLine(list, inner.Min.X, covY, line, covColor)
	}

	// Interactive indicator (if applicable)
	if f.entry.Interactive {
		icon := "⌖"
		iconStyle := f.th.TextStyle(theme.TextLabelS)
		iconLayout := f.shaper.ShapeSimple(icon, iconStyle)
		if iconLayout != nil && len(iconLayout.Lines) > 0 {
			line := iconLayout.Lines[0]
			iconX := inner.Max.X - line.Bounds.Width()
			iconY := inner.Max.Y - iconLayout.Bounds.Height()
			f.drawTextLine(list, iconX, iconY, line, f.th.Color(theme.ColorPrimary))
		}
	}
}

// truncateText truncates text if it exceeds max width, adding ellipsis.
func (f *CardFacet) truncateText(s string, maxWidth float32, token theme.TextToken) string {
	if f.shaper == nil || len(s) == 0 {
		return s
	}

	style := f.th.TextStyle(token)
	layout := f.shaper.ShapeSimple(s, style)
	if layout == nil || len(layout.Lines) == 0 {
		return s
	}

	// Check if text needs truncation:
	// - Multiple lines (wrapped)
	// - Single line wider than maxWidth
	needsTruncation := len(layout.Lines) > 1
	if !needsTruncation && len(layout.Lines) == 1 {
		needsTruncation = layout.Lines[0].Bounds.Width() > maxWidth
	}

	if !needsTruncation {
		return s
	}

	// Binary search for best truncation point
	ellipsis := "..."
	low, high := 0, len(s)
	for low < high {
		mid := (low + high + 1) / 2
		test := s[:mid] + ellipsis
		testLayout := f.shaper.ShapeSimple(test, style)
		if testLayout == nil || len(testLayout.Lines) == 0 {
			high = mid - 1
			continue
		}
		// Must fit: single line and within max width
		lineCount := len(testLayout.Lines)
		lineWidth := float32(0)
		if lineCount > 0 {
			lineWidth = testLayout.Lines[0].Bounds.Width()
		}
		fits := lineCount == 1 && lineWidth <= maxWidth
		if fits {
			low = mid
		} else {
			high = mid - 1
		}
	}
	if low > 0 {
		result := s[:low] + ellipsis
		return result
	}
	// If even single char + ellipsis doesn't fit, just return ellipsis
	return ellipsis
}

func (f *CardFacet) drawTextLine(list *gfx.CommandList, x, y float32, line text.ShapedLine, color gfx.Color) {
	origin := gfx.Point{X: x + line.Bounds.Min.X, Y: y + line.Baseline}
	for _, run := range line.Runs {
		list.Add(gfx.DrawGlyphRun{
			Run:    run,
			Origin: origin,
			Brush:  gfx.SolidBrush(color),
		})
	}
}
