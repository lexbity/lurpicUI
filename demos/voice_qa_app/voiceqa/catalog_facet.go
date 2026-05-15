package voiceqa

import (
	"fmt"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
)

// CatalogFacet shows the voice mark/facet/theme registry to aid QA.
type CatalogFacet struct {
	facet.Facet
	layout facet.LayoutRole
	render facet.RenderRole

	host   *Host
	th     theme.Context
	shaper *text.Shaper
	bounds gfx.Rect
}

func NewCatalogFacet(host *Host, th theme.Context, shaper *text.Shaper) *CatalogFacet {
	c := &CatalogFacet{
		Facet:  facet.NewFacet(),
		host:   host,
		th:     th,
		shaper: shaper,
	}
	c.layout.OnMeasure = func(facet.Constraints) gfx.Size { return gfx.Size{W: 360, H: 220} }
	c.layout.OnArrange = func(bounds gfx.Rect) { c.bounds = bounds }
	c.render.OnCollect = func(list *gfx.CommandList, bounds gfx.Rect) { c.renderCatalog(list, bounds) }
	c.AddRole(&c.layout)
	c.AddRole(&c.render)
	return c
}

func (c *CatalogFacet) Base() *facet.Facet {
	c.Facet.BindImpl(c)
	return &c.Facet
}

func (c *CatalogFacet) OnAttach(ctx facet.AttachContext) {}
func (c *CatalogFacet) OnDetach()                        {}
func (c *CatalogFacet) OnActivate()                      {}
func (c *CatalogFacet) OnDeactivate()                    {}

func (c *CatalogFacet) renderCatalog(list *gfx.CommandList, bounds gfx.Rect) {
	if list == nil || bounds.IsEmpty() {
		return
	}
	list.Add(gfx.FillRect{Rect: bounds, Brush: gfx.SolidBrush(gfx.ColorFromHex(0x11161CFF))})
	if c.host == nil || c.host.DescriptorRegistry() == nil || c.shaper == nil {
		return
	}
	reg := c.host.DescriptorRegistry()
	marks := reg.Marks()
	facets := reg.Facets()
	themes := reg.ThemeSlots()

	y := bounds.Min.Y + 10
	c.draw(list, bounds.Min.X+10, y, "Voice UX catalog", theme.TextLabelS, theme.ColorText)
	y += 18
	c.draw(list, bounds.Min.X+10, y, fmt.Sprintf("marks: %d  facets: %d  themes: %d", len(marks), len(facets), len(themes)), theme.TextBodyS, theme.ColorTextSecondary)
	y += 18
	c.draw(list, bounds.Min.X+10, y, "Marks:", theme.TextBodyS, theme.ColorTextSecondary)
	y += 18
	for i, m := range marks {
		if i >= 4 {
			break
		}
		c.draw(list, bounds.Min.X+10, y, string(m.Type), theme.TextBodyS, theme.ColorText)
		y += 16
	}
	y += 8
	c.draw(list, bounds.Min.X+10, y, "Facets:", theme.TextBodyS, theme.ColorTextSecondary)
	y += 18
	for i, f := range facets {
		if i >= 3 {
			break
		}
		c.draw(list, bounds.Min.X+10, y, f.ID+" - "+f.Name, theme.TextBodyS, theme.ColorText)
		y += 16
	}
}

func (c *CatalogFacet) draw(list *gfx.CommandList, x, y float32, label string, style theme.TextToken, color theme.ColorToken) {
	if c.shaper == nil || label == "" {
		return
	}
	layout := c.shaper.ShapeSimple(label, c.th.TextStyle(style))
	if layout == nil || len(layout.Lines) == 0 {
		return
	}
	line := layout.Lines[0]
	origin := gfx.Point{X: x, Y: y + line.Baseline}
	for _, run := range line.Runs {
		list.Add(gfx.DrawGlyphRun{Run: run, Origin: origin, Brush: gfx.SolidBrush(c.th.Color(color))})
	}
}
