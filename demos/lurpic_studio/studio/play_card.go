package studio

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/primitive"
	"codeburg.org/lexbit/lurpicui/theme"
)

// playCard is a bespoke card host for the E6 playground (Finding
// F-card-content). The framework structure.Card draws its content children by
// self-projection without attaching them to the facet tree, so marks inside a
// Card are never hit-testable by the runtime — an interactive exercise mark
// cannot live in one. This host attaches the exercise mark(s) as real
// facet-tree children (so they project and receive input) and draws the card
// chrome (background, border, title) itself.
//
// Body marks are arranged in a single horizontal row of equal columns; the
// title sits above them. Colors resolve from the runtime theme at arrange time.
type playCard struct {
	facet.Facet
	layout facet.LayoutRole
	render facet.RenderRole

	title *primitive.Text
	body  []facet.FacetImpl //lurpiclint:ignore LL012 -- the hosted exercise marks are composition structure, not domain state (F-lint-hosts)

	bg     gfx.Color
	border gfx.Color
	textCl gfx.Color
	padX   float32
	padY   float32
	gap    float32
	radius float32
}

// newPlayCard builds a card hosting the given exercise mark(s).
func newPlayCard(title string, body ...facet.FacetImpl) *playCard {
	c := &playCard{
		title:  primitive.NewText(marks.Const(title)),
		body:   append([]facet.FacetImpl(nil), body...),
		padX:   12,
		padY:   10,
		gap:    8,
		radius: 6,
	}
	c.Facet = facet.NewFacet()
	c.AddChild(c.title.Base()) //lurpiclint:ignore LL021 -- E6 hosts a card title as a regular child, not an overlay (LL021 over-fires)
	for _, b := range c.body {
		if b != nil && b.Base() != nil {
			c.AddChild(b.Base()) //lurpiclint:ignore LL021 -- E6 hosts playground marks as regular children, not overlays (LL021 over-fires)
		}
	}

	c.layout = facet.LayoutRole{ //lurpiclint:ignore * -- bespoke card host (F-lint-hosts)
		OnMeasure: func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
			return c.measure(ctx, constraints)
		},
		OnArrange: func(ctx facet.ArrangeContext, bounds gfx.Rect) {
			c.arrange(ctx, bounds)
		},
	}
	c.layout.Child = linearChildContract(facet.StretchPolicy{
		Width:  facet.StretchAlways,
		Height: facet.StretchAlways,
	})
	c.render = facet.RenderRole{
		OnCollect: func(list *gfx.CommandList, bounds gfx.Rect) {
			if bounds.IsEmpty() {
				return
			}
			list.Add(gfx.FillRect{Rect: bounds, Brush: gfx.SolidBrush(c.bg)})
			if c.radius > 0 {
				list.Add(gfx.StrokePath{Path: gfx.RoundedRectPath(bounds, c.radius), Brush: gfx.SolidBrush(c.border), Stroke: gfx.StrokeStyle{Width: 1}})
			}
		},
	}
	c.AddRole(&c.layout)
	c.AddRole(&c.render)
	return c
}

// measure measures the title and the body marks against the available width,
// then sizes the card to content height.
func (c *playCard) measure(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
	avail := gfx.Size{W: constraints.MaxSize.W}
	titleH := float32(0)
	if role := c.title.Base().LayoutRole(); role != nil {
		role.Measure(ctx, facet.Constraints{MaxSize: avail})
		titleH = role.MeasuredSize.H
	}
	bodyH := float32(0)
	if len(c.body) > 0 {
		colW := c.bodyColumnWidth(constraints.MaxSize.W)
		for _, b := range c.body {
			if b == nil || b.Base() == nil || b.Base().LayoutRole() == nil {
				continue
			}
			b.Base().LayoutRole().Measure(ctx, facet.Constraints{MaxSize: gfx.Size{W: colW}})
			h := b.Base().LayoutRole().MeasuredSize.H
			if h > bodyH {
				bodyH = h
			}
		}
	}
	height := c.padY*2 + titleH
	if bodyH > 0 {
		height += c.gap + bodyH
	}
	measured := constraints.Constrain(gfx.Size{W: constraints.MaxSize.W, H: height})
	return facet.MeasureResult{Size: measured}
}

// arrange places the title above the body row and stores the resolved theme
// colors for the render role.
func (c *playCard) arrange(ctx facet.ArrangeContext, bounds gfx.Rect) {
	if resolved, ok := ctx.Theme.(theme.ResolvedContext); ok {
		c.bg = resolved.Color(theme.ColorSurface)
		c.border = resolved.Color(theme.ColorBorder)
		c.textCl = resolved.Color(theme.ColorText)
	}
	if bounds.IsEmpty() {
		if role := c.title.Base().LayoutRole(); role != nil {
			role.Arrange(ctx, gfx.Rect{})
		}
		for _, b := range c.body {
			if b != nil && b.Base() != nil && b.Base().LayoutRole() != nil {
				b.Base().LayoutRole().Arrange(ctx, gfx.Rect{})
			}
		}
		return
	}
	titleH := float32(0)
	if role := c.title.Base().LayoutRole(); role != nil {
		titleH = role.MeasuredSize.H
		role.Arrange(ctx, gfx.RectFromXYWH(bounds.Min.X+c.padX, bounds.Min.Y+c.padY, bounds.Width()-c.padX*2, titleH))
	}
	cursorY := bounds.Min.Y + c.padY*2 + titleH
	if len(c.body) > 0 {
		cursorY += c.gap
	}
	if len(c.body) > 0 {
		colW := c.bodyColumnWidth(bounds.Width())
		x := bounds.Min.X + c.padX
		for _, b := range c.body {
			if b == nil || b.Base() == nil || b.Base().LayoutRole() == nil {
				continue
			}
			b.Base().LayoutRole().Arrange(ctx, gfx.RectFromXYWH(x, cursorY, colW, bounds.Max.Y-cursorY-c.padY))
			x += colW + c.gap
		}
	}
}

// bodyColumnWidth is the equal-column width for the body marks.
func (c *playCard) bodyColumnWidth(avail float32) float32 {
	n := len(c.body)
	if n <= 0 {
		return avail - c.padX*2
	}
	return (avail - c.padX*2 - c.gap*float32(n-1)) / float32(n)
}

func (c *playCard) Base() *facet.Facet             { c.BindImpl(c); return &c.Facet }
func (c *playCard) OnAttach(_ facet.AttachContext) {}
func (c *playCard) OnDetach()                      {}
func (c *playCard) OnActivate()                    {}
func (c *playCard) OnDeactivate()                  {}
