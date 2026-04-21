package annotation

import (
	"sync"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
)

// CalloutDirection controls where the callout body is placed.
type CalloutDirection uint8

const (
	CalloutAbove CalloutDirection = iota
	CalloutBelow
	CalloutLeft
	CalloutRight
)

// Callout is a floating annotation body with an optional leader line.
type Callout struct {
	ID        string
	Target    AnchorSourceRef
	Body      marks.Mark
	Direction CalloutDirection
	Offset    gfx.Point
	WithLine  bool

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
		ConstructionClass: marks.ConstructionComposed,
		Type:              marks.TypeName("annotation:callout"),
		HitTestable:       true,
		AnchorExporting:   true,
	})
}

func (c *Callout) Base() *facet.Facet { c.ensureInit(); return &c.base }
func (c *Callout) Descriptor() marks.Descriptor {
	return marks.Descriptor{Family: marks.FamilyAnnotation, ConstructionClass: marks.ConstructionComposed, Type: marks.TypeName("annotation:callout"), HitTestable: true, AnchorExporting: true}
}
func (c *Callout) AuthoredID() string { return c.ID }
func (c *Callout) OnAttach(ctx facet.AttachContext) { c.syncRoles() }
func (c *Callout) OnDetach() {}
func (c *Callout) OnActivate() {}
func (c *Callout) OnDeactivate() {}

func (c *Callout) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	c.ensureInit()
	bounds := c.bodyBounds()
	if bounds.IsEmpty() {
		return nil
	}
	transform := gfx.Translation(c.resolvedPosition().X, c.resolvedPosition().Y)
	if ctx.Viewport != (layout.Viewport{}) {
		transform = ctx.Viewport.Transform.Multiply(transform)
	}
	return transformAnchors(transform, boundsAnchors(bounds))
}

func (c *Callout) ensureInit() {
	c.once.Do(func() {
		c.base.BindImpl(c)
		c.layoutRole = &facet.LayoutRole{OnMeasure: func(cn facet.Constraints) gfx.Size {
			bounds := c.bodyBounds()
			return gfx.Size{W: bounds.Width(), H: bounds.Height()}
		}}
		c.viewportRole = &facet.ViewportRole{Transform: gfx.Identity()}
		c.projection = &facet.ProjectionRole{OnProject: func(ctx facet.ProjectionContext) *gfx.CommandList { return c.project(ctx) }}
		c.hitRole = &facet.HitRole{OnHitTest: func(p gfx.Point) facet.HitResult {
			if c.bodyBounds().Contains(p) {
				return facet.HitResult{Hit: true, Cursor: facet.CursorDefault}
			}
			return facet.HitResult{}
		}}
		c.base.AddRole(c.layoutRole)
		c.base.AddRole(c.viewportRole)
		c.base.AddRole(c.projection)
		c.base.AddRole(c.hitRole)
		c.syncRoles()
	})
}

func (c *Callout) syncRoles() {
	syncLayout(c.layoutRole, c.bodyBounds())
	syncViewport(c.viewportRole, gfx.Translation(c.resolvedPosition().X, c.resolvedPosition().Y))
}

func (c *Callout) bodyBounds() gfx.Rect {
	size := gfx.RectFromXYWH(-48, -24, 96, 48)
	switch c.Direction {
	case CalloutAbove:
		return gfx.RectFromXYWH(size.Min.X, size.Min.Y-32, size.Width(), size.Height())
	case CalloutBelow:
		return gfx.RectFromXYWH(size.Min.X, size.Min.Y+32, size.Width(), size.Height())
	case CalloutLeft:
		return gfx.RectFromXYWH(size.Min.X-48, size.Min.Y, size.Width(), size.Height())
	case CalloutRight:
		return gfx.RectFromXYWH(size.Min.X+48, size.Min.Y, size.Width(), size.Height())
	default:
		return size
	}
}

func (c *Callout) project(ctx facet.ProjectionContext) *gfx.CommandList {
	bounds := c.bodyBounds()
	var list gfx.CommandList
	list.Add(gfx.PushTransform{Matrix: gfx.Translation(c.resolvedPosition().X, c.resolvedPosition().Y)})
	list.Add(gfx.FillRect{Rect: bounds, Brush: gfx.SolidBrush(gfx.Color{A: 0.95})})
	if c.WithLine {
		target := gfx.Point{}
		body := boundsAnchors(bounds)["bounds-center"]
		list.Add(gfx.StrokePath{
			Path:   pathFromPoints([]gfx.Point{target, body}, false),
			Stroke: gfx.DefaultStroke(1.2),
			Brush:  gfx.SolidBrush(gfx.Color{A: 1}),
		})
	}
	if c.Body != nil {
		if cmds := projectMarkAt(c.Body, boundsAnchors(bounds)["bounds-center"], ctx); cmds != nil {
			list.Commands = append(list.Commands, cmds.Commands...)
		}
	}
	list.Add(gfx.PopTransform{})
	return &list
}

func (c *Callout) resolvedPosition() gfx.Point {
	if root := c.base.Parent(); root != nil {
		if pt, ok := anchorPoint(root, c.Target, "bounds-center"); ok {
			return gfx.Point{X: pt.X + c.Offset.X, Y: pt.Y + c.Offset.Y}
		}
	}
	return c.Offset
}

func (c *Callout) targetPoint() gfx.Point {
	if root := c.base.Parent(); root != nil {
		if pt, ok := anchorPoint(root, c.Target, "bounds-center"); ok {
			return pt
		}
	}
	return gfx.Point{}
}
