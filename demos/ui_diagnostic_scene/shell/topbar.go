package shell

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
)

// TopBarFacet displays metadata (theme, backend, platform, build info)
type TopBarFacet struct {
	facet.Facet
	layout facet.LayoutRole
	render facet.RenderRole
	theme  theme.Context
	shaper *text.Shaper
	text   string
}

// NewTopBarFacet constructs the top metadata bar
func NewTopBarFacet(th theme.Context, shaper *text.Shaper) *TopBarFacet {
	t := &TopBarFacet{
		Facet:  facet.NewFacet(),
		theme:  th,
		shaper: shaper,
		text:   "UI Diagnostic Scene | Theme: Default | Backend: Software | Platform: Linux",
	}

	t.layout.OnMeasure = func(c facet.Constraints) gfx.Size {
		return gfx.Size{W: c.MaxSize.W, H: 32}
	}
	t.layout.OnArrange = func(bounds gfx.Rect) {
		t.layout.ArrangedBounds = bounds
	}
	t.AddRole(&t.layout)

	t.render.OnCollect = func(list *gfx.CommandList, bounds gfx.Rect) {
		t.renderTopBar(list, bounds)
	}
	t.AddRole(&t.render)

	return t
}

func (t *TopBarFacet) Base() *facet.Facet {
	t.Facet.BindImpl(t)
	return &t.Facet
}

func (t *TopBarFacet) OnAttach(ctx facet.AttachContext) {}
func (t *TopBarFacet) OnDetach()                        {}
func (t *TopBarFacet) OnActivate()                      {}
func (t *TopBarFacet) OnDeactivate()                    {}

func (t *TopBarFacet) renderTopBar(list *gfx.CommandList, bounds gfx.Rect) {
	if list == nil || bounds.IsEmpty() {
		return
	}

	// Background
	list.Add(gfx.FillRect{
		Rect:  bounds,
		Brush: gfx.SolidBrush(t.theme.Color(theme.ColorSurface)),
	})

	// Bottom border
	list.Add(gfx.FillRect{
		Rect:  gfx.RectFromXYWH(bounds.Min.X, bounds.Max.Y-1, bounds.Width(), 1),
		Brush: gfx.SolidBrush(t.theme.Color(theme.ColorBorder)),
	})

	if t.shaper == nil {
		return
	}

	// Text
	style := t.theme.TextStyle(theme.TextLabelS)
	layout := t.shaper.ShapeSimple(t.text, style)
	if layout != nil && len(layout.Lines) > 0 {
		line := layout.Lines[0]
		origin := gfx.Point{X: bounds.Min.X + 12, Y: bounds.Min.Y + 20}
		for _, run := range line.Runs {
			list.Add(gfx.DrawGlyphRun{
				Run:    run,
				Origin: origin,
				Brush:  gfx.SolidBrush(t.theme.Color(theme.ColorText)),
			})
		}
	}
}

// SetInfo updates the displayed metadata
func (t *TopBarFacet) SetInfo(themeName, backend, platform string) {
	t.text = "UI Diagnostic Scene | Theme: " + themeName + " | Backend: " + backend + " | Platform: " + platform
	t.Invalidate(facet.DirtyProjection)
}
