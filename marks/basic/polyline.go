package basic

import (
	"sync"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/theme"
)

// Polyline is a primitive open path authored mark.
type Polyline struct {
	ID     string
	Points []gfx.Point
	Stroke theme.MaterialStroke
	Tx     TransformProps

	base           primitiveFacet
	once           sync.Once
	layoutRole     *facet.LayoutRole
	viewportRole   *facet.ViewportRole
	projectionRole *facet.ProjectionRole
	hitRole        *facet.HitRole
}

func init() {
	registerPrimitiveDescriptor(marks.Descriptor{
		Family:            marks.FamilyBasic,
		ConstructionClass: marks.ConstructionPrimitive,
		Type:              marks.TypeName("basic:polyline"),
		HitTestable:       true,
		AnchorExporting:   true,
	})
}

func (p *Polyline) Base() *facet.Facet               { p.ensureInit(); return p.base.Base() }
func (p *Polyline) Descriptor() marks.Descriptor     { p.ensureInit(); return p.base.Descriptor() }
func (p *Polyline) AuthoredID() string               { return p.ID }
func (p *Polyline) OnAttach(ctx facet.AttachContext) { p.syncRoles() }
func (p *Polyline) OnDetach()                        {}
func (p *Polyline) OnActivate()                      {}
func (p *Polyline) OnDeactivate()                    {}

func (p *Polyline) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	p.ensureInit()
	if len(p.Points) == 0 {
		return nil
	}
	anchors := layout.AnchorSet{
		"start": {X: p.Points[0].X, Y: p.Points[0].Y},
		"end":   {X: p.Points[len(p.Points)-1].X, Y: p.Points[len(p.Points)-1].Y},
	}
	transform := normalizeTransform(p.Tx.Transform)
	if ctx.Viewport != (layout.Viewport{}) {
		transform = ctx.Viewport.Transform.Multiply(transform)
	}
	return transformAnchors(transform, anchors)
}

func (p *Polyline) HitTest(world gfx.Point) bool {
	p.ensureInit()
	inv, ok := inverseTransform(p.Tx)
	if !ok {
		return false
	}
	return p.hitTestLocal(inv.TransformPoint(world))
}

func (p *Polyline) ensureInit() {
	p.once.Do(func() {
		p.base.descriptor = marks.Descriptor{Family: marks.FamilyBasic, ConstructionClass: marks.ConstructionPrimitive, Type: marks.TypeName("basic:polyline"), HitTestable: true, AnchorExporting: true}
		p.layoutRole = &facet.LayoutRole{OnMeasure: func(c facet.Constraints) gfx.Size {
			bounds := p.localBounds()
			return gfx.Size{W: bounds.Width(), H: bounds.Height()}
		}}
		p.viewportRole = &facet.ViewportRole{Transform: normalizeTransform(p.Tx.Transform)}
		p.projectionRole = &facet.ProjectionRole{OnProject: func(ctx facet.ProjectionContext) *gfx.CommandList { return p.project(ctx) }}
		p.hitRole = &facet.HitRole{OnHitTest: func(pt gfx.Point) facet.HitResult {
			if p.hitTestLocal(pt) {
				return facet.HitResult{Hit: true, Cursor: facet.CursorDefault}
			}
			return facet.HitResult{}
		}}
		attachPrimitiveRoles(&p.base, p.layoutRole, p.viewportRole, p.projectionRole, p.hitRole)
		syncLayout(p.layoutRole, p.localBounds())
		syncViewport(p.viewportRole, normalizeTransform(p.Tx.Transform))
	})
}

func (p *Polyline) syncRoles() {
	syncLayout(p.layoutRole, p.localBounds())
	syncViewport(p.viewportRole, normalizeTransform(p.Tx.Transform))
}

func (p *Polyline) localBounds() gfx.Rect {
	if len(p.Points) == 0 {
		return gfx.Rect{}
	}
	minX, maxX := p.Points[0].X, p.Points[0].X
	minY, maxY := p.Points[0].Y, p.Points[0].Y
	for _, pt := range p.Points[1:] {
		if pt.X < minX {
			minX = pt.X
		}
		if pt.X > maxX {
			maxX = pt.X
		}
		if pt.Y < minY {
			minY = pt.Y
		}
		if pt.Y > maxY {
			maxY = pt.Y
		}
	}
	pad := p.Stroke.Width / 2
	return gfx.RectFromXYWH(minX-pad, minY-pad, (maxX-minX)+pad*2, (maxY-minY)+pad*2)
}

func (p *Polyline) project(ctx facet.ProjectionContext) *gfx.CommandList {
	if p.Stroke.Width <= 0 || len(p.Points) < 2 {
		return &gfx.CommandList{}
	}
	var list gfx.CommandList
	list.Add(gfx.DrawPolyline{Points: append([]gfx.Point(nil), p.Points...), Stroke: strokeStyle(p.Stroke), Brush: strokeBrushFromMaterial(p.Stroke, 1), Closed: false})
	return &list
}

func (p *Polyline) hitTestLocal(pt gfx.Point) bool {
	if len(p.Points) < 2 {
		return false
	}
	tol := p.Stroke.Width / 2
	if tol <= 0 {
		tol = 0.5
	}
	for i := 1; i < len(p.Points); i++ {
		if segmentDistance(pt, p.Points[i-1], p.Points[i]) <= tol {
			return true
		}
	}
	return false
}
