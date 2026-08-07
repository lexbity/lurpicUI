package studio

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/layout/split"
)

// dividerSize is the static gutter width between panes (R-split-drag: static
// gutters only, no draggable dividers).
const dividerSize = 4

// Pane describes one split pane: the hosted facet plus its split sizing
// hints. Flex > 0 makes the pane weighted (width distributed by flex weight);
// FixedWidth > 0 makes it a fixed main-axis pane; otherwise it is intrinsic.
type Pane struct {
	Facet      facet.FacetImpl
	Flex       float32
	FixedWidth float32
	MinWidth   float32
}

// GallerySplit is the bespoke 3-pane split host (index | stage | inspector).
//
// It is the first production consumer of the layout/split policy. Per F-split
// there is no GroupLayoutSplit kind, so the host cannot use the group-parent
// bridge: it wires []layout.ChildNode itself and drives split.Measure and
// split.Arrange from its own LayoutRole, capturing each pane's arranged
// bounds through a ChildArrangeHandle.
type GallerySplit struct {
	facet.Facet
	layout facet.LayoutRole
	render facet.RenderRole

	panes   []Pane
	divider float32

	dividerColor gfx.Color
}

// NewGallerySplit builds a split host for the given panes and static gutter
// width.
func NewGallerySplit(panes []Pane, divider float32) *GallerySplit {
	g := &GallerySplit{
		panes:        append([]Pane(nil), panes...),
		divider:      divider,
		dividerColor: gfx.Color{R: 0.85, G: 0.85, B: 0.88, A: 1},
	}
	g.Facet = facet.NewFacet()
	for i := range g.panes {
		if g.panes[i].Facet != nil {
			g.AddChild(g.panes[i].Facet.Base())
		}
	}

	g.layout = facet.LayoutRole{ //lurpiclint:ignore * -- bespoke split-pane host (F-split)
		OnMeasure: func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
			return g.measure(ctx, c)
		},
		OnArrange: func(ctx facet.ArrangeContext, bounds gfx.Rect) {
			g.arrange(ctx, bounds)
		},
	}
	g.layout.Child = facet.GroupChildContract{
		SupportedPlacement: facet.SupportsGrid | facet.SupportsLinear,
		Stretch: facet.StretchPolicy{
			Width:  facet.StretchAlways,
			Height: facet.StretchAlways,
		},
	}
	g.render = facet.RenderRole{
		OnCollect: func(list *gfx.CommandList, bounds gfx.Rect) {
			list.Commands = append(list.Commands, g.dividerCommands(bounds)...)
		},
	}
	g.AddRole(&g.layout)
	g.AddRole(&g.render)
	return g
}

// Panes returns the split's pane list.
func (g *GallerySplit) Panes() []Pane { return append([]Pane(nil), g.panes...) }

// childNodes produces the split ChildNodes for the panes from their current
// measured sizes. When fillCross > 0 the cross axis (the split's height) is
// forced to that value so the panes fill the split's full height on arrange.
func (g *GallerySplit) childNodes(avail gfx.Size, fillCross float32) []layout.ChildNode {
	nodes := make([]layout.ChildNode, 0, len(g.panes))
	for _, p := range g.panes {
		if p.Facet == nil || p.Facet.Base() == nil {
			continue
		}
		role := p.Facet.Base().LayoutRole()
		if role == nil {
			continue
		}
		is := role.MeasuredSize
		min := gfx.Size{W: p.MinWidth, H: is.H}
		if fillCross > 0 {
			is.H = fillCross
			min.H = fillCross
		}
		nodes = append(nodes, layout.ChildNode{
			FacetID: p.Facet.Base().ID(),
			Attachment: layout.ChildAttachment{
				Placement: layout.PlacementHints{
					Flex:   p.Flex,
					Offset: gfx.Point{X: p.FixedWidth},
				},
			},
			IntrinsicSize: is,
			MinSize:       min,
		})
	}
	return nodes
}

func (g *GallerySplit) measure(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
	// Measure panes with an unbounded cross (MaxSize.H = 0 is the framework's
	// "no clamp" sentinel) so Cards report their content height instead of
	// flex-filling the whole split; the split then stretches to the residual
	// through Root's linear MainAxisMax.
	avail := gfx.Size{W: c.MaxSize.W}
	for _, p := range g.panes {
		if p.Facet == nil || p.Facet.Base() == nil {
			continue
		}
		if role := p.Facet.Base().LayoutRole(); role != nil {
			role.Measure(ctx, facet.Constraints{MaxSize: avail})
		}
	}
	nodes := g.childNodes(avail, 0)
	policy := split.New(split.Config{Axis: split.Horizontal, DividerSize: g.divider})
	return facet.MeasureResult{Size: policy.Measure(nodes, c.MaxSize)}
}

func (g *GallerySplit) arrange(ctx facet.ArrangeContext, bounds gfx.Rect) {
	if bounds.IsEmpty() {
		return
	}
	nodes := g.childNodes(gfx.Size{W: bounds.Width(), H: bounds.Height()}, bounds.Height())
	handles := make([]*layout.ChildArrangeHandle, len(nodes))
	for i := range nodes {
		h := &layout.ChildArrangeHandle{}
		nodes[i].AttachArrangeHandle(h)
		handles[i] = h
	}
	policy := split.New(split.Config{Axis: split.Horizontal, DividerSize: g.divider})
	policy.Arrange(nodes, layout.ResolvedLayer{Bounds: bounds})
	for i := range g.panes {
		if i >= len(handles) || g.panes[i].Facet == nil {
			continue
		}
		if b, ok := handles[i].Bounds(); ok {
			g.panes[i].Facet.Base().LayoutRole().Arrange(facet.ArrangeContext{
				Placement: facet.Placement{Mode: facet.PlacementGrid},
			}, b)
		}
	}
}

// dividerCommands draws the static gutters between consecutive panes from
// their arranged bounds.
func (g *GallerySplit) dividerCommands(bounds gfx.Rect) []gfx.Command {
	var cmds []gfx.Command
	for i := 0; i+1 < len(g.panes); i++ {
		if g.panes[i].Facet == nil || g.panes[i+1].Facet == nil {
			continue
		}
		left := g.panes[i].Facet.Base().LayoutRole().ArrangedBounds
		if left.IsEmpty() {
			continue
		}
		cmds = append(cmds, gfx.FillRect{
			Rect:  gfx.RectFromXYWH(left.Max.X, bounds.Min.Y, g.divider, bounds.Height()),
			Brush: gfx.SolidBrush(g.dividerColor),
		})
	}
	return cmds
}

// Children returns the split's group children (the ChildSource the host would
// expose if it used the group-parent bridge; kept for the shell's contract).
func (g *GallerySplit) Children() []facet.GroupChild {
	out := make([]facet.GroupChild, 0, len(g.panes))
	for i, p := range g.panes {
		if p.Facet == nil || p.Facet.Base() == nil {
			continue
		}
		role := p.Facet.Base().LayoutRole()
		if role == nil {
			continue
		}
		out = append(out, facet.GroupChild{
			FacetID:    p.Facet.Base().ID(),
			Attachment: facet.Attachment{Placement: facet.Placement{Mode: facet.PlacementGrid}},
			Layout:     role,
			Contract:   role.Child,
		})
		_ = i
	}
	return out
}

func (g *GallerySplit) Base() *facet.Facet             { g.BindImpl(g); return &g.Facet }
func (g *GallerySplit) OnAttach(_ facet.AttachContext) {}
func (g *GallerySplit) OnDetach()                      {}
func (g *GallerySplit) OnActivate()                    {}
func (g *GallerySplit) OnDeactivate()                  {}
