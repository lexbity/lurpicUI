package annotation

import (
	"sync"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/basic"
	"codeburg.org/lexbit/lurpicui/theme"
)

// AreaMode selects the source geometry for an area fill.
type AreaMode uint8

const (
	AreaFromBaseline AreaMode = iota
	AreaBetweenContours
)

// Area is a generated fill region.
type Area struct {
	ID       string
	Mode     AreaMode
	PointsA  []gfx.Point
	PointsB  []gfx.Point
	Baseline float32
	Style    basic.PrimitiveStyleProps

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
		ConstructionClass: marks.ConstructionGenerated,
		Type:              marks.TypeName("annotation:area"),
		HitTestable:       true,
		AnchorExporting:   true,
	})
}

func (a *Area) Base() *facet.Facet { a.ensureInit(); return &a.base }
func (a *Area) Descriptor() marks.Descriptor {
	return marks.Descriptor{Family: marks.FamilyAnnotation, ConstructionClass: marks.ConstructionGenerated, Type: marks.TypeName("annotation:area"), HitTestable: true, AnchorExporting: true}
}
func (a *Area) AuthoredID() string { return a.ID }
func (a *Area) OnAttach(ctx facet.AttachContext) { a.syncRoles() }
func (a *Area) OnDetach() {}
func (a *Area) OnActivate() {}
func (a *Area) OnDeactivate() {}

func (a *Area) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	a.ensureInit()
	bounds := a.localBounds()
	if bounds.IsEmpty() {
		return nil
	}
	transform := gfx.Identity()
	if ctx.Viewport != (layout.Viewport{}) {
		transform = ctx.Viewport.Transform
	}
	return transformAnchors(transform, boundsAnchors(bounds))
}

func (a *Area) ensureInit() {
	a.once.Do(func() {
		a.base.BindImpl(a)
		a.layoutRole = &facet.LayoutRole{OnMeasure: func(c facet.Constraints) gfx.Size {
			bounds := a.localBounds()
			return gfx.Size{W: bounds.Width(), H: bounds.Height()}
		}}
		a.viewportRole = &facet.ViewportRole{Transform: gfx.Identity()}
		a.projection = &facet.ProjectionRole{OnProject: func(ctx facet.ProjectionContext) *gfx.CommandList { return a.project(ctx) }}
		a.hitRole = &facet.HitRole{OnHitTest: func(p gfx.Point) facet.HitResult {
			if a.hitTestLocal(p) {
				return facet.HitResult{Hit: true, Cursor: facet.CursorDefault}
			}
			return facet.HitResult{}
		}}
		a.base.AddRole(a.layoutRole)
		a.base.AddRole(a.viewportRole)
		a.base.AddRole(a.projection)
		a.base.AddRole(a.hitRole)
		a.syncRoles()
	})
}

func (a *Area) syncRoles() {
	syncLayout(a.layoutRole, a.localBounds())
	syncViewport(a.viewportRole, gfx.Identity())
}

func (a *Area) localPath() gfx.Path {
	switch a.Mode {
	case AreaBetweenContours:
		if len(a.PointsA) == 0 || len(a.PointsB) == 0 {
			return gfx.Path{}
		}
		pts := append([]gfx.Point(nil), a.PointsA...)
		for i := len(a.PointsB) - 1; i >= 0; i-- {
			pts = append(pts, a.PointsB[i])
		}
		return pathFromPoints(pts, true)
	default:
		if len(a.PointsA) == 0 {
			return gfx.Path{}
		}
		pts := append([]gfx.Point(nil), a.PointsA...)
		if len(pts) > 0 {
			last := pts[len(pts)-1]
			first := pts[0]
			pts = append(pts, gfx.Point{X: last.X, Y: a.Baseline}, gfx.Point{X: first.X, Y: a.Baseline})
		}
		return pathFromPoints(pts, true)
	}
}

func (a *Area) localBounds() gfx.Rect {
	return pathBounds(a.localPath())
}

func (a *Area) project(ctx facet.ProjectionContext) *gfx.CommandList {
	path := a.localPath()
	if len(path.Segments) == 0 {
		return &gfx.CommandList{}
	}
	var list gfx.CommandList
	material := a.Style
	if material.Opacity <= 0 {
		material.Opacity = 1
	}
	for _, fill := range material.Fill.Fills {
		if fill.Type != theme.FillNone {
			list.Add(gfx.FillPath{Path: path, Brush: gfx.SolidBrush(fill.Color)})
		}
	}
	if material.Stroke.Width > 0 {
		list.Add(gfx.StrokePath{Path: path, Stroke: strokeStyle(material.Stroke), Brush: strokeBrushFromMaterial(material.Stroke, 1)})
	}
	return &list
}

func (a *Area) hitTestLocal(p gfx.Point) bool {
	path := a.localPath()
	if len(path.Segments) == 0 {
		return false
	}
	return pathContains(path, p, true)
}
