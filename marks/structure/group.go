package structure

import (
	"sync"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
)

// Group is a composition-first authored mark that hosts child marks under a shared transform.
type Group struct {
	ID        string
	Transform gfx.Transform
	Children  []marks.Mark
	Visible   bool

	base           facet.Facet
	once           sync.Once
	layoutRole     *facet.LayoutRole
	viewportRole   *facet.ViewportRole
	projectionRole *facet.ProjectionRole
}

func init() {
	registerStructureDescriptor(marks.Descriptor{
		Family:            marks.FamilyStructure,
		ConstructionClass: marks.ConstructionComposed,
		Type:              marks.TypeName("structure:group"),
		AnchorExporting:   true,
		ChildHosting:      true,
	})
}

func (g *Group) Base() *facet.Facet { g.ensureInit(); return &g.base }

func (g *Group) Descriptor() marks.Descriptor {
	g.ensureInit()
	return marks.Descriptor{
		Family:            marks.FamilyStructure,
		ConstructionClass: marks.ConstructionComposed,
		Type:              marks.TypeName("structure:group"),
		AnchorExporting:   true,
		ChildHosting:      true,
	}
}

func (g *Group) AuthoredID() string { return g.ID }
func (g *Group) OnAttach(ctx facet.AttachContext) {
	g.syncRoles()
}
func (g *Group) OnDetach()     {}
func (g *Group) OnActivate()   {}
func (g *Group) OnDeactivate() {}

func (g *Group) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	g.ensureInit()
	bounds, ok := unionDescendantBounds(&g.base)
	if !ok {
		return nil
	}
	anchors := boundsAnchors(bounds)
	transform := normaliseTransform(g.Transform)
	if ctx.Viewport != (layout.Viewport{}) {
		transform = ctx.Viewport.Transform.Multiply(transform)
	}
	return transformAnchors(transform, anchors)
}

func (g *Group) ensureInit() {
	g.once.Do(func() {
		if g.base.ID() == 0 {
			g.base = facet.NewFacet()
		}
		g.base.BindImpl(g)
		g.layoutRole = &facet.LayoutRole{
			OnMeasure: func(c facet.Constraints) gfx.Size {
				bounds, ok := unionDescendantBounds(&g.base)
				if !ok {
					return gfx.Size{}
				}
				return gfx.Size{W: bounds.Width(), H: bounds.Height()}
			},
			OnArrange: func(bounds gfx.Rect) {
				g.layoutRole.ArrangedBounds = bounds
				origin := bounds.Min
				for _, child := range g.base.Children() {
					if child == nil {
						continue
					}
					lr := child.LayoutRole()
					if lr == nil {
						continue
					}
					size := lr.Measure(layout.Loose(gfx.Size{W: bounds.Width(), H: bounds.Height()}))
					if size.W <= 0 || size.H <= 0 {
						size = lr.MeasuredSize
					}
					if size.W <= 0 {
						size.W = bounds.Width()
					}
					if size.H <= 0 {
						size.H = bounds.Height()
					}
					lr.Arrange(gfx.RectFromXYWH(origin.X, origin.Y, size.W, size.H))
				}
			},
		}
		g.viewportRole = &facet.ViewportRole{Transform: normaliseTransform(g.Transform)}
		g.projectionRole = &facet.ProjectionRole{OnProject: func(ctx facet.ProjectionContext) *gfx.CommandList {
			return &gfx.CommandList{}
		}}
		g.base.AddRole(g.layoutRole)
		g.base.AddRole(g.viewportRole)
		g.base.AddRole(g.projectionRole)
		attachChildMarks(&g.base, g.Children)
		syncLayout(g.layoutRole, g.localBounds())
		syncViewport(g.viewportRole, normaliseTransform(g.Transform))
	})
}

func (g *Group) syncRoles() {
	syncLayout(g.layoutRole, g.localBounds())
	syncViewport(g.viewportRole, normaliseTransform(g.Transform))
}

func (g *Group) localBounds() gfx.Rect {
	bounds, ok := unionDescendantBounds(&g.base)
	if !ok {
		return gfx.Rect{}
	}
	return bounds
}
