package studio

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/primitive"
	"codeburg.org/lexbit/lurpicui/marks/status"
	"codeburg.org/lexbit/lurpicui/theme"
)

// StatusBar is the bottom status strip: a connection badge and a status
// caption. Like ChromeStack it is a linear-kind group-parent host that
// arranges its mark children directly (F-linear-marks).
type StatusBar struct {
	facet.Facet
	layout facet.LayoutRole
	render facet.RenderRole

	badge   facet.FacetImpl
	caption facet.FacetImpl

	gap        float32
	padX       float32
	padY       float32
	background gfx.Color
}

// NewStatusBar builds the status strip for the given resolved theme.
func NewStatusBar(themeCtx theme.ResolvedContext) *StatusBar {
	s := &StatusBar{
		badge:      status.NewBadge("ready"),
		caption:    primitive.NewText(marks.Const("gallery shell — slice 2")),
		gap:        float32(themeCtx.Spacing(theme.SpacingM)),
		padX:       float32(themeCtx.Spacing(theme.SpacingL)),
		padY:       float32(themeCtx.Spacing(theme.SpacingXS)),
		background: themeCtx.Color(theme.ColorSurfaceVariant),
	}
	s.Facet = facet.NewFacet()
	s.AddChild(s.badge.Base())
	s.AddChild(s.caption.Base())

	s.layout = facet.LayoutRole{ //lurpiclint:ignore * -- bespoke linear-kind group-parent host (F-lint-hosts)
		OnMeasure: func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
			return s.measure(ctx, constraints)
		},
		OnArrange: func(ctx facet.ArrangeContext, bounds gfx.Rect) {
			s.arrange(ctx, bounds)
		},
	}
	s.layout.Parent = facet.GroupParentContract{
		Kind:     facet.GroupLayoutLinearHorizontal,
		Policy:   groupPolicy{kind: facet.GroupLayoutLinearHorizontal, host: s},
		Children: s,
	}
	s.layout.Child = linearChildContract(facet.StretchPolicy{
		Width:  facet.StretchAlways,
		Height: facet.StretchNever,
	})
	s.render = facet.RenderRole{
		OnCollect: func(list *gfx.CommandList, bounds gfx.Rect) {
			list.Add(gfx.FillRect{Rect: bounds, Brush: gfx.SolidBrush(s.background)})
		},
	}
	s.AddRole(&s.layout)
	s.AddRole(&s.render)
	return s
}

// Badge returns the connection badge facet.
func (s *StatusBar) Badge() facet.FacetImpl { return s.badge }

// Caption returns the status caption facet.
func (s *StatusBar) Caption() facet.FacetImpl { return s.caption }

func (s *StatusBar) measure(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
	items := []facet.FacetImpl{s.badge, s.caption}
	width := s.padX * 2
	height := float32(0)
	for i, item := range items {
		role := item.Base().LayoutRole()
		role.Measure(ctx, facet.Constraints{MaxSize: constraints.MaxSize})
		size := role.MeasuredSize
		width += size.W
		if i < len(items)-1 {
			width += s.gap
		}
		if size.H > height {
			height = size.H
		}
	}
	height += s.padY * 2
	return facet.MeasureResult{Size: gfx.Size{W: width, H: height}}
}

func (s *StatusBar) arrange(ctx facet.ArrangeContext, bounds gfx.Rect) {
	if bounds.IsEmpty() {
		return
	}
	items := []facet.FacetImpl{s.badge, s.caption}
	x := bounds.Min.X + s.padX
	for _, item := range items {
		role := item.Base().LayoutRole()
		w := role.MeasuredSize.W
		arrangeChild(facet.ArrangeContext{}, item, gfx.RectFromXYWH(x, bounds.Min.Y, w, bounds.Height()))
		x += w + s.gap
	}
}

// Children returns the status bar's group children.
func (s *StatusBar) Children() []facet.GroupChild {
	return linearGroupChildren([]facet.FacetImpl{s.badge, s.caption})
}

func (s *StatusBar) Base() *facet.Facet             { s.BindImpl(s); return &s.Facet }
func (s *StatusBar) OnAttach(_ facet.AttachContext) {}
func (s *StatusBar) OnDetach()                      {}
func (s *StatusBar) OnActivate()                    {}
func (s *StatusBar) OnDeactivate()                  {}
