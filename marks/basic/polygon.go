package basic

import (
	"sync"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/theme"
)

// Polygon is a primitive closed polygon authored mark.
type Polygon struct {
	ID       string
	Points   []gfx.Point
	FillRule FillRule
	Style    PrimitiveStyleProps
	Tx       TransformProps

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
		Type:              marks.TypeName("basic:polygon"),
		HitTestable:       true,
		AnchorExporting:   true,
	})
}

func (p *Polygon) Base() *facet.Facet               { p.ensureInit(); return p.base.Base() }
func (p *Polygon) Descriptor() marks.Descriptor     { p.ensureInit(); return p.base.Descriptor() }
func (p *Polygon) AuthoredID() string               { return p.ID }
func (p *Polygon) OnAttach(ctx facet.AttachContext) { p.syncRoles() }
func (p *Polygon) OnDetach()                        {}
func (p *Polygon) OnActivate()                      {}
func (p *Polygon) OnDeactivate()                    {}

func (p *Polygon) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	p.ensureInit()
	if len(p.Points) < 3 {
		return nil
	}
	anchors := p.boundsAnchors()
	if c := p.centroid(); c != nil {
		anchors["centroid"] = *c
	}
	transform := normalizeTransform(p.Tx.Transform)
	if ctx.Viewport != (layout.Viewport{}) {
		transform = ctx.Viewport.Transform.Multiply(transform)
	}
	return transformAnchors(transform, anchors)
}

func (p *Polygon) HitTest(world gfx.Point) bool {
	p.ensureInit()
	inv, ok := inverseTransform(p.Tx)
	if !ok {
		return false
	}
	return p.hitTestLocal(inv.TransformPoint(world))
}

func (p *Polygon) ensureInit() {
	p.once.Do(func() {
		p.base.descriptor = marks.Descriptor{Family: marks.FamilyBasic, ConstructionClass: marks.ConstructionPrimitive, Type: marks.TypeName("basic:polygon"), HitTestable: true, AnchorExporting: true}
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

func (p *Polygon) syncRoles() {
	syncLayout(p.layoutRole, p.localBounds())
	syncViewport(p.viewportRole, normalizeTransform(p.Tx.Transform))
}

func (p *Polygon) localBounds() gfx.Rect {
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
	pad := max(p.Style.Stroke.Width/2, 0)
	return gfx.RectFromXYWH(minX-pad, minY-pad, (maxX-minX)+pad*2, (maxY-minY)+pad*2)
}

func (p *Polygon) project(ctx facet.ProjectionContext) *gfx.CommandList {
	if len(p.Points) < 3 || !emptyStyleVisible(p.Style) {
		return &gfx.CommandList{}
	}
	path := gfx.PolylinePath(append([]gfx.Point(nil), p.Points...), true)
	var list gfx.CommandList
	for _, fill := range p.Style.Fill.Fills {
		if fill.Type == theme.FillNone {
			continue
		}
		list.Add(gfx.FillPath{Path: path, Brush: colorBrush(fill, p.Style.Opacity)})
	}
	if p.Style.Stroke.Width > 0 {
		list.Add(gfx.StrokePath{Path: path, Stroke: strokeStyle(p.Style.Stroke), Brush: strokeBrushFromMaterial(p.Style.Stroke, p.Style.Opacity)})
	}
	return &list
}

func (p *Polygon) hitTestLocal(pt gfx.Point) bool {
	if len(p.Points) < 3 {
		return false
	}
	if len(p.Style.Fill.Fills) > 0 && p.Style.Fill.Fills[0].Type != theme.FillNone {
		if pathContains(gfx.PolylinePath(append([]gfx.Point(nil), p.Points...), true), pt, p.FillRule == FillRuleEvenOdd) {
			return true
		}
	}
	if p.Style.Stroke.Width > 0 {
		path := gfx.PolylinePath(append([]gfx.Point(nil), p.Points...), true)
		if pathStrokeHit(path, pt, p.Style.Stroke.Width) {
			return true
		}
	}
	return false
}

func (p *Polygon) boundsAnchors() layout.AnchorSet {
	bounds := p.localBounds()
	if bounds.IsEmpty() {
		return nil
	}
	return layout.AnchorSet{
		"bounds-center": {X: bounds.Min.X + bounds.Width()/2, Y: bounds.Min.Y + bounds.Height()/2},
		"top-left":      {X: bounds.Min.X, Y: bounds.Min.Y},
		"top-right":     {X: bounds.Max.X, Y: bounds.Min.Y},
		"bottom-right":  {X: bounds.Max.X, Y: bounds.Max.Y},
		"bottom-left":   {X: bounds.Min.X, Y: bounds.Max.Y},
	}
}

func (p *Polygon) centroid() *gfx.Point {
	if len(p.Points) < 3 {
		return nil
	}
	var area, cx, cy float32
	for i := 0; i < len(p.Points); i++ {
		j := (i + 1) % len(p.Points)
		cross := p.Points[i].X*p.Points[j].Y - p.Points[j].X*p.Points[i].Y
		area += cross
		cx += (p.Points[i].X + p.Points[j].X) * cross
		cy += (p.Points[i].Y + p.Points[j].Y) * cross
	}
	if area == 0 {
		return nil
	}
	area *= 0.5
	cx /= 6 * area
	cy /= 6 * area
	pt := gfx.Point{X: cx, Y: cy}
	return &pt
}
