package basic

import (
	"sync"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/theme"
)

// Line is a primitive line segment authored mark.
type Line struct {
	ID     string
	Start  gfx.Point
	End    gfx.Point
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
		Type:              marks.TypeName("basic:line"),
		HitTestable:       true,
		AnchorExporting:   true,
	})
}

// Base returns the embedded facet base.
func (l *Line) Base() *facet.Facet {
	l.ensureInit()
	return l.base.Base()
}

// Descriptor returns the line descriptor.
func (l *Line) Descriptor() marks.Descriptor {
	l.ensureInit()
	return l.base.Descriptor()
}

// AuthoredID returns the authored identifier.
func (l *Line) AuthoredID() string { return l.ID }

func (l *Line) OnAttach(ctx facet.AttachContext) { l.syncRoles() }
func (l *Line) OnDetach()                        {}
func (l *Line) OnActivate()                      {}
func (l *Line) OnDeactivate()                    {}

// ExportAnchors exports line anchors in world space.
func (l *Line) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	l.ensureInit()
	anchors := layout.AnchorSet{
		"start": {X: l.Start.X, Y: l.Start.Y},
		"mid":   {X: (l.Start.X + l.End.X) / 2, Y: (l.Start.Y + l.End.Y) / 2},
		"end":   {X: l.End.X, Y: l.End.Y},
	}
	transform := normalizeTransform(l.Tx.Transform)
	if ctx.Viewport != (layout.Viewport{}) {
		transform = ctx.Viewport.Transform.Multiply(transform)
	}
	return transformAnchors(transform, anchors)
}

// HitTest returns whether the supplied world point lies within the line stroke.
func (l *Line) HitTest(world gfx.Point) bool {
	l.ensureInit()
	inv, ok := inverseTransform(l.Tx)
	if !ok {
		return false
	}
	return l.hitTestLocal(inv.TransformPoint(world))
}

func (l *Line) ensureInit() {
	l.once.Do(func() {
		l.base.descriptor = marks.Descriptor{
			Family:            marks.FamilyBasic,
			ConstructionClass: marks.ConstructionPrimitive,
			Type:              marks.TypeName("basic:line"),
			HitTestable:       true,
			AnchorExporting:   true,
		}
		l.layoutRole = &facet.LayoutRole{
			OnMeasure: func(c facet.Constraints) gfx.Size {
				bounds := l.localBounds()
				return gfx.Size{W: bounds.Width(), H: bounds.Height()}
			},
			OnArrange: func(bounds gfx.Rect) {
				l.Start = bounds.Min
				l.End = bounds.Max
			},
		}
		l.viewportRole = &facet.ViewportRole{Transform: normalizeTransform(l.Tx.Transform)}
		l.projectionRole = &facet.ProjectionRole{OnProject: func(ctx facet.ProjectionContext) *gfx.CommandList {
			return l.project(ctx)
		}}
		l.hitRole = &facet.HitRole{OnHitTest: func(p gfx.Point) facet.HitResult {
			if l.hitTestLocal(p) {
				return facet.HitResult{Hit: true, Cursor: facet.CursorDefault}
			}
			return facet.HitResult{}
		}}
		attachPrimitiveRoles(&l.base, l.layoutRole, l.viewportRole, l.projectionRole, l.hitRole)
		syncLayout(l.layoutRole, l.localBounds())
		syncViewport(l.viewportRole, normalizeTransform(l.Tx.Transform))
	})
}

func (l *Line) syncRoles() {
	syncLayout(l.layoutRole, l.localBounds())
	syncViewport(l.viewportRole, normalizeTransform(l.Tx.Transform))
}

func (l *Line) localBounds() gfx.Rect {
	minX := min(l.Start.X, l.End.X)
	minY := min(l.Start.Y, l.End.Y)
	maxX := max(l.Start.X, l.End.X)
	maxY := max(l.Start.Y, l.End.Y)
	pad := l.Stroke.Width / 2
	return gfx.RectFromXYWH(minX-pad, minY-pad, (maxX-minX)+pad*2, (maxY-minY)+pad*2)
}

func (l *Line) project(ctx facet.ProjectionContext) *gfx.CommandList {
	if l.Stroke.Width <= 0 {
		return &gfx.CommandList{}
	}
	points := []gfx.Point{l.Start, l.End}
	var list gfx.CommandList
	list.Add(gfx.DrawPolyline{
		Points: points,
		Stroke: strokeStyle(l.Stroke),
		Brush:  strokeBrushFromMaterial(l.Stroke, 1),
	})
	return &list
}

func (l *Line) hitTestLocal(p gfx.Point) bool {
	tolerance := l.Stroke.Width / 2
	if tolerance <= 0 {
		tolerance = 0.5
	}
	return segmentDistance(p, l.Start, l.End) <= tolerance
}
