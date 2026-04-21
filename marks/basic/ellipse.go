package basic

import (
	"sync"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/theme"
)

// Ellipse is a primitive elliptical authored mark.
type Ellipse struct {
	ID     string
	Bounds BoundsProps
	Style  PrimitiveStyleProps
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
		Type:              marks.TypeName("basic:ellipse"),
		HitTestable:       true,
		AnchorExporting:   true,
	})
}

// Base returns the embedded facet base.
func (e *Ellipse) Base() *facet.Facet {
	e.ensureInit()
	return e.base.Base()
}

// Descriptor returns the ellipse descriptor.
func (e *Ellipse) Descriptor() marks.Descriptor {
	e.ensureInit()
	return e.base.Descriptor()
}

// AuthoredID returns the authored identifier.
func (e *Ellipse) AuthoredID() string { return e.ID }

// OnAttach syncs the facet roles with the current public fields.
func (e *Ellipse) OnAttach(ctx facet.AttachContext) { e.syncRoles() }

func (e *Ellipse) OnDetach()     {}
func (e *Ellipse) OnActivate()   {}
func (e *Ellipse) OnDeactivate() {}

// ExportAnchors exports the ellipse anchors in world space.
func (e *Ellipse) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	e.ensureInit()
	bounds := e.localBounds()
	anchors := layout.AnchorSet{
		"center": {X: bounds.Min.X + bounds.Width()/2, Y: bounds.Min.Y + bounds.Height()/2},
		"north":  {X: bounds.Min.X + bounds.Width()/2, Y: bounds.Min.Y},
		"east":   {X: bounds.Max.X, Y: bounds.Min.Y + bounds.Height()/2},
		"south":  {X: bounds.Min.X + bounds.Width()/2, Y: bounds.Max.Y},
		"west":   {X: bounds.Min.X, Y: bounds.Min.Y + bounds.Height()/2},
	}
	transform := normalizeTransform(e.Tx.Transform)
	if ctx.Viewport != (layout.Viewport{}) {
		transform = ctx.Viewport.Transform.Multiply(transform)
	}
	return transformAnchors(transform, anchors)
}

// HitTest returns whether the supplied world point lies within the ellipse.
func (e *Ellipse) HitTest(world gfx.Point) bool {
	e.ensureInit()
	inv, ok := inverseTransform(e.Tx)
	if !ok {
		return false
	}
	return e.hitTestLocal(inv.TransformPoint(world))
}

func (e *Ellipse) ensureInit() {
	e.once.Do(func() {
		e.base.descriptor = marks.Descriptor{
			Family:            marks.FamilyBasic,
			ConstructionClass: marks.ConstructionPrimitive,
			Type:              marks.TypeName("basic:ellipse"),
			HitTestable:       true,
			AnchorExporting:   true,
		}
		e.layoutRole = &facet.LayoutRole{
			OnMeasure: func(c facet.Constraints) gfx.Size {
				bounds := e.localBounds()
				return gfx.Size{W: bounds.Width(), H: bounds.Height()}
			},
			OnArrange: func(bounds gfx.Rect) {
				e.Bounds = BoundsProps{X: bounds.Min.X, Y: bounds.Min.Y, W: bounds.Width(), H: bounds.Height()}
			},
		}
		e.viewportRole = &facet.ViewportRole{Transform: normalizeTransform(e.Tx.Transform)}
		e.projectionRole = &facet.ProjectionRole{OnProject: func(ctx facet.ProjectionContext) *gfx.CommandList {
			return e.project(ctx)
		}}
		e.hitRole = &facet.HitRole{OnHitTest: func(p gfx.Point) facet.HitResult {
			if e.hitTestLocal(p) {
				return facet.HitResult{Hit: true, Cursor: facet.CursorDefault}
			}
			return facet.HitResult{}
		}}
		attachPrimitiveRoles(&e.base, e.layoutRole, e.viewportRole, e.projectionRole, e.hitRole)
		syncLayout(e.layoutRole, e.localBounds())
		syncViewport(e.viewportRole, normalizeTransform(e.Tx.Transform))
	})
}

func (e *Ellipse) syncRoles() {
	syncLayout(e.layoutRole, e.localBounds())
	syncViewport(e.viewportRole, normalizeTransform(e.Tx.Transform))
}

func (e *Ellipse) localBounds() gfx.Rect {
	return e.Bounds.Rect()
}

func (e *Ellipse) project(ctx facet.ProjectionContext) *gfx.CommandList {
	if !emptyStyleVisible(e.Style) {
		return &gfx.CommandList{}
	}
	bounds := ctx.Bounds
	if bounds.IsEmpty() {
		bounds = e.localBounds()
	}
	path := ellipsePath(bounds)
	var list gfx.CommandList
	for _, fill := range e.Style.Fill.Fills {
		if fill.Type == theme.FillNone {
			continue
		}
		list.Add(gfx.FillPath{Path: path, Brush: colorBrush(fill, e.Style.Opacity)})
	}
	if e.Style.Stroke.Width > 0 {
		stroke := e.Style.Stroke
		list.Add(gfx.StrokePath{Path: path, Stroke: strokeStyle(stroke), Brush: strokeBrushFromMaterial(stroke, e.Style.Opacity)})
	}
	return &list
}

func (e *Ellipse) hitTestLocal(p gfx.Point) bool {
	bounds := e.localBounds()
	if bounds.IsEmpty() {
		return false
	}
	cx := bounds.Min.X + bounds.Width()/2
	cy := bounds.Min.Y + bounds.Height()/2
	rx := bounds.Width() / 2
	ry := bounds.Height() / 2
	if rx <= 0 || ry <= 0 {
		return false
	}
	nx := (p.X - cx) / rx
	ny := (p.Y - cy) / ry
	return nx*nx+ny*ny <= 1
}

func ellipsePath(bounds gfx.Rect) gfx.Path {
	if bounds.IsEmpty() {
		return gfx.Path{}
	}
	cx := bounds.Min.X + bounds.Width()/2
	cy := bounds.Min.Y + bounds.Height()/2
	rx := bounds.Width() / 2
	ry := bounds.Height() / 2
	k := float32(0.552284749831)
	kx := rx * k
	ky := ry * k
	return gfx.NewPath().
		MoveTo(gfx.Point{X: cx + rx, Y: cy}).
		CubicTo(
			gfx.Point{X: cx + rx, Y: cy + ky},
			gfx.Point{X: cx + kx, Y: cy + ry},
			gfx.Point{X: cx, Y: cy + ry},
		).
		CubicTo(
			gfx.Point{X: cx - kx, Y: cy + ry},
			gfx.Point{X: cx - rx, Y: cy + ky},
			gfx.Point{X: cx - rx, Y: cy},
		).
		CubicTo(
			gfx.Point{X: cx - rx, Y: cy - ky},
			gfx.Point{X: cx - kx, Y: cy - ry},
			gfx.Point{X: cx, Y: cy - ry},
		).
		CubicTo(
			gfx.Point{X: cx + kx, Y: cy - ry},
			gfx.Point{X: cx + rx, Y: cy - ky},
			gfx.Point{X: cx + rx, Y: cy},
		).
		Close().
		Build()
}
