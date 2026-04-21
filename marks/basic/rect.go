package basic

import (
	"sync"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/theme"
)

// Rect is a primitive rectangular authored mark.
type Rect struct {
	ID     string
	Bounds BoundsProps
	Radius float32
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
		Type:              marks.TypeName("basic:rect"),
		HitTestable:       true,
		AnchorExporting:   true,
	})
}

// Base returns the embedded facet base.
func (r *Rect) Base() *facet.Facet {
	r.ensureInit()
	return r.base.Base()
}

// Descriptor returns the rect descriptor.
func (r *Rect) Descriptor() marks.Descriptor {
	r.ensureInit()
	return r.base.Descriptor()
}

// AuthoredID returns the authored identifier.
func (r *Rect) AuthoredID() string {
	return r.ID
}

// OnAttach syncs the facet roles with the current public fields.
func (r *Rect) OnAttach(ctx facet.AttachContext) {
	r.syncRoles()
}

// OnDetach is a no-op.
func (r *Rect) OnDetach() {}

// OnActivate is a no-op.
func (r *Rect) OnActivate() {}

// OnDeactivate is a no-op.
func (r *Rect) OnDeactivate() {}

// ExportAnchors exports the common rect anchors in world space.
func (r *Rect) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	r.ensureInit()
	bounds := r.localBounds()
	anchors := layout.AnchorSet{
		"center":       {X: bounds.Min.X + bounds.Width()/2, Y: bounds.Min.Y + bounds.Height()/2},
		"top-left":     {X: bounds.Min.X, Y: bounds.Min.Y},
		"top":          {X: bounds.Min.X + bounds.Width()/2, Y: bounds.Min.Y},
		"top-right":    {X: bounds.Max.X, Y: bounds.Min.Y},
		"right":        {X: bounds.Max.X, Y: bounds.Min.Y + bounds.Height()/2},
		"bottom-right": {X: bounds.Max.X, Y: bounds.Max.Y},
		"bottom":       {X: bounds.Min.X + bounds.Width()/2, Y: bounds.Max.Y},
		"bottom-left":  {X: bounds.Min.X, Y: bounds.Max.Y},
		"left":         {X: bounds.Min.X, Y: bounds.Min.Y + bounds.Height()/2},
	}
	transform := normalizeTransform(r.Tx.Transform)
	if ctx.Viewport != (layout.Viewport{}) {
		transform = ctx.Viewport.Transform.Multiply(transform)
	}
	return transformAnchors(transform, anchors)
}

// HitTest returns whether the supplied world point lies within the rect.
func (r *Rect) HitTest(world gfx.Point) bool {
	r.ensureInit()
	local, ok := inverseTransform(r.Tx)
	if !ok {
		return false
	}
	return r.hitTestLocal(local.TransformPoint(world))
}

func (r *Rect) ensureInit() {
	r.once.Do(func() {
		r.base.descriptor = marks.Descriptor{
			Family:            marks.FamilyBasic,
			ConstructionClass: marks.ConstructionPrimitive,
			Type:              marks.TypeName("basic:rect"),
			HitTestable:       true,
			AnchorExporting:   true,
		}
		r.layoutRole = &facet.LayoutRole{
			OnMeasure: func(c facet.Constraints) gfx.Size {
				bounds := r.localBounds()
				return gfx.Size{W: bounds.Width(), H: bounds.Height()}
			},
			OnArrange: func(bounds gfx.Rect) {
				r.Bounds = BoundsProps{X: bounds.Min.X, Y: bounds.Min.Y, W: bounds.Width(), H: bounds.Height()}
			},
		}
		r.viewportRole = &facet.ViewportRole{Transform: normalizeTransform(r.Tx.Transform)}
		r.projectionRole = &facet.ProjectionRole{OnProject: func(ctx facet.ProjectionContext) *gfx.CommandList {
			return r.project(ctx)
		}}
		r.hitRole = &facet.HitRole{OnHitTest: func(p gfx.Point) facet.HitResult {
			if r.hitTestLocal(p) {
				return facet.HitResult{Hit: true, Cursor: facet.CursorDefault}
			}
			return facet.HitResult{}
		}}
		attachPrimitiveRoles(&r.base, r.layoutRole, r.viewportRole, r.projectionRole, r.hitRole)
		syncLayout(r.layoutRole, r.localBounds())
		syncViewport(r.viewportRole, normalizeTransform(r.Tx.Transform))
	})
}

func (r *Rect) syncRoles() {
	syncLayout(r.layoutRole, r.localBounds())
	syncViewport(r.viewportRole, normalizeTransform(r.Tx.Transform))
}

func (r *Rect) localBounds() gfx.Rect {
	return r.Bounds.Rect()
}

func (r *Rect) project(ctx facet.ProjectionContext) *gfx.CommandList {
	if !emptyStyleVisible(r.Style) {
		return &gfx.CommandList{}
	}
	bounds := ctx.Bounds
	if bounds.IsEmpty() {
		bounds = r.localBounds()
	}
	var list gfx.CommandList
	if r.Radius <= 0 {
		for _, fill := range r.Style.Fill.Fills {
			if fill.Type == theme.FillNone {
				continue
			}
			list.Add(gfx.FillRect{Rect: bounds, Brush: colorBrush(fill, r.Style.Opacity)})
		}
		if r.Style.Stroke.Width > 0 {
			stroke := r.Style.Stroke
			list.Add(gfx.StrokeRect{Rect: bounds, Stroke: strokeStyle(stroke), Brush: strokeBrushFromMaterial(stroke, r.Style.Opacity)})
		}
		return &list
	}
	path := rectPath(bounds, r.Radius)
	for _, fill := range r.Style.Fill.Fills {
		if fill.Type == theme.FillNone {
			continue
		}
		list.Add(gfx.FillPath{Path: path, Brush: colorBrush(fill, r.Style.Opacity)})
	}
	if r.Style.Stroke.Width > 0 {
		stroke := r.Style.Stroke
		strokeStyle := strokeStyle(stroke)
		list.Add(gfx.StrokePath{Path: path, Stroke: strokeStyle, Brush: strokeBrushFromMaterial(stroke, r.Style.Opacity)})
	}
	return &list
}

func (r *Rect) hitTestLocal(p gfx.Point) bool {
	bounds := r.localBounds()
	if bounds.IsEmpty() {
		return false
	}
	if r.Radius <= 0 {
		return bounds.Contains(p)
	}
	radius := r.Radius
	maxRadius := min(bounds.Width(), bounds.Height()) / 2
	if radius > maxRadius {
		radius = maxRadius
	}
	if p.X < bounds.Min.X || p.X > bounds.Max.X || p.Y < bounds.Min.Y || p.Y > bounds.Max.Y {
		return false
	}
	left := bounds.Min.X + radius
	right := bounds.Max.X - radius
	top := bounds.Min.Y + radius
	bottom := bounds.Max.Y - radius
	if p.X >= left && p.X <= right {
		return true
	}
	if p.Y >= top && p.Y <= bottom {
		return true
	}
	corners := []gfx.Point{
		{X: left, Y: top},
		{X: right, Y: top},
		{X: right, Y: bottom},
		{X: left, Y: bottom},
	}
	for _, corner := range corners {
		if distance(p, corner) <= radius {
			return true
		}
	}
	return false
}

func rectPath(bounds gfx.Rect, radius float32) gfx.Path {
	if radius <= 0 {
		return gfx.RectPath(bounds)
	}
	return gfx.RoundedRectPath(bounds, radius)
}
