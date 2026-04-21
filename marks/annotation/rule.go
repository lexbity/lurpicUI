package annotation

import (
	"math"
	"sync"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/theme"
)

// Rule is a semantic line mark.
type Rule struct {
	ID     string
	Start  gfx.Point
	End    gfx.Point
	Stroke theme.MaterialStroke
	Inset  float32

	base         facet.Facet
	once         sync.Once
	layoutRole   *facet.LayoutRole
	viewportRole *facet.ViewportRole
	projection   *facet.ProjectionRole
	hitRole      *facet.HitRole
}

func init() {
	registerAnnotationDescriptor(marks.Descriptor{
		Family:            marks.FamilyAnnotation,
		ConstructionClass: marks.ConstructionPrimitive,
		Type:              marks.TypeName("annotation:rule"),
		HitTestable:       true,
		AnchorExporting:   true,
	})
}

func (r *Rule) Base() *facet.Facet { r.ensureInit(); return &r.base }

func (r *Rule) Descriptor() marks.Descriptor {
	return marks.Descriptor{
		Family:            marks.FamilyAnnotation,
		ConstructionClass: marks.ConstructionPrimitive,
		Type:              marks.TypeName("annotation:rule"),
		HitTestable:       true,
		AnchorExporting:   true,
	}
}

func (r *Rule) AuthoredID() string { return r.ID }
func (r *Rule) OnAttach(ctx facet.AttachContext) {
	r.syncRoles()
}
func (r *Rule) OnDetach()     {}
func (r *Rule) OnActivate()   {}
func (r *Rule) OnDeactivate() {}

func (r *Rule) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	r.ensureInit()
	anchors := layout.AnchorSet{
		"start": {X: r.Start.X, Y: r.Start.Y},
		"mid":   {X: (r.Start.X + r.End.X) / 2, Y: (r.Start.Y + r.End.Y) / 2},
		"end":   {X: r.End.X, Y: r.End.Y},
	}
	transform := gfx.Identity()
	if ctx.Viewport != (layout.Viewport{}) {
		transform = ctx.Viewport.Transform
	}
	return transformAnchors(transform, anchors)
}

func (r *Rule) HitTest(world gfx.Point) bool {
	r.ensureInit()
	return r.hitTestLocal(world)
}

func (r *Rule) ensureInit() {
	r.once.Do(func() {
		r.base.BindImpl(r)
		r.layoutRole = &facet.LayoutRole{
			OnMeasure: func(c facet.Constraints) gfx.Size {
				bounds := r.localBounds()
				return gfx.Size{W: bounds.Width(), H: bounds.Height()}
			},
		}
		r.viewportRole = &facet.ViewportRole{Transform: gfx.Identity()}
		r.projection = &facet.ProjectionRole{OnProject: func(ctx facet.ProjectionContext) *gfx.CommandList { return r.project(ctx) }}
		r.hitRole = &facet.HitRole{OnHitTest: func(p gfx.Point) facet.HitResult {
			if r.hitTestLocal(p) {
				return facet.HitResult{Hit: true, Cursor: facet.CursorDefault}
			}
			return facet.HitResult{}
		}}
		r.base.AddRole(r.layoutRole)
		r.base.AddRole(r.viewportRole)
		r.base.AddRole(r.projection)
		r.base.AddRole(r.hitRole)
		syncLayout(r.layoutRole, r.localBounds())
		syncViewport(r.viewportRole, gfx.Identity())
	})
}

func (r *Rule) syncRoles() {
	syncLayout(r.layoutRole, r.localBounds())
	syncViewport(r.viewportRole, gfx.Identity())
}

func (r *Rule) localBounds() gfx.Rect {
	minX := min(r.Start.X, r.End.X)
	minY := min(r.Start.Y, r.End.Y)
	maxX := max(r.Start.X, r.End.X)
	maxY := max(r.Start.Y, r.End.Y)
	pad := r.Stroke.Width/2 + r.Inset
	return gfx.RectFromXYWH(minX-pad, minY-pad, (maxX-minX)+pad*2, (maxY-minY)+pad*2)
}

func (r *Rule) project(ctx facet.ProjectionContext) *gfx.CommandList {
	if r.Stroke.Width <= 0 {
		return &gfx.CommandList{}
	}
	start, end := r.adjustedEndpoints()
	var list gfx.CommandList
	list.Add(gfx.DrawPolyline{
		Points: []gfx.Point{start, end},
		Stroke: strokeStyle(r.Stroke),
		Brush:  strokeBrushFromMaterial(r.Stroke, 1),
	})
	return &list
}

func (r *Rule) adjustedEndpoints() (gfx.Point, gfx.Point) {
	dx := r.End.X - r.Start.X
	dy := r.End.Y - r.Start.Y
	length := float32(math.Hypot(float64(dx), float64(dy)))
	if length == 0 || r.Inset <= 0 {
		return r.Start, r.End
	}
	inset := r.Inset
	if inset*2 > length {
		inset = length / 2
	}
	ux := dx / length
	uy := dy / length
	return gfx.Point{X: r.Start.X + ux*inset, Y: r.Start.Y + uy*inset},
		gfx.Point{X: r.End.X - ux*inset, Y: r.End.Y - uy*inset}
}

func (r *Rule) hitTestLocal(p gfx.Point) bool {
	return segmentDistance(p, r.Start, r.End) <= max(0.5, r.Stroke.Width/2)
}
