package annotation

import (
	"sync"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
)

// Badge is a host-attached semantic badge.
type Badge struct {
	ID      string
	Host    AnchorSourceRef
	Content marks.Mark
	Offset  gfx.Point

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
		Type:              marks.TypeName("annotation:badge"),
		HitTestable:       true,
		AnchorExporting:   true,
	})
}

func (b *Badge) Base() *facet.Facet { b.ensureInit(); return &b.base }
func (b *Badge) Descriptor() marks.Descriptor {
	return marks.Descriptor{Family: marks.FamilyAnnotation, ConstructionClass: marks.ConstructionComposed, Type: marks.TypeName("annotation:badge"), HitTestable: true, AnchorExporting: true}
}
func (b *Badge) AuthoredID() string { return b.ID }
func (b *Badge) OnAttach(ctx facet.AttachContext) { b.syncRoles() }
func (b *Badge) OnDetach() {}
func (b *Badge) OnActivate() {}
func (b *Badge) OnDeactivate() {}

func (b *Badge) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	b.ensureInit()
	bounds := b.localBounds()
	if bounds.IsEmpty() {
		return nil
	}
	transform := gfx.Translation(b.resolvedPosition().X, b.resolvedPosition().Y)
	if ctx.Viewport != (layout.Viewport{}) {
		transform = ctx.Viewport.Transform.Multiply(transform)
	}
	return transformAnchors(transform, boundsAnchors(bounds))
}

func (b *Badge) ensureInit() {
	b.once.Do(func() {
		b.base.BindImpl(b)
		b.layoutRole = &facet.LayoutRole{OnMeasure: func(c facet.Constraints) gfx.Size {
			bounds := b.localBounds()
			return gfx.Size{W: bounds.Width(), H: bounds.Height()}
		}}
		b.viewportRole = &facet.ViewportRole{Transform: gfx.Identity()}
		b.projection = &facet.ProjectionRole{OnProject: func(ctx facet.ProjectionContext) *gfx.CommandList { return b.project(ctx) }}
		b.hitRole = &facet.HitRole{OnHitTest: func(p gfx.Point) facet.HitResult {
			if b.localBounds().Contains(p) {
				return facet.HitResult{Hit: true, Cursor: facet.CursorDefault}
			}
			return facet.HitResult{}
		}}
		b.base.AddRole(b.layoutRole)
		b.base.AddRole(b.viewportRole)
		b.base.AddRole(b.projection)
		b.base.AddRole(b.hitRole)
		b.syncRoles()
	})
}

func (b *Badge) syncRoles() {
	syncLayout(b.layoutRole, b.localBounds())
	syncViewport(b.viewportRole, gfx.Translation(b.resolvedPosition().X, b.resolvedPosition().Y))
}

func (b *Badge) localBounds() gfx.Rect {
	return gfx.RectFromXYWH(-12, -8, 24, 16)
}

func (b *Badge) project(ctx facet.ProjectionContext) *gfx.CommandList {
	bounds := b.localBounds()
	var list gfx.CommandList
	list.Add(gfx.PushTransform{Matrix: gfx.Translation(b.resolvedPosition().X, b.resolvedPosition().Y)})
	list.Add(gfx.FillRect{Rect: bounds, Brush: gfx.SolidBrush(gfx.Color{A: 0.9})})
	if b.Content != nil {
		if pt, ok := b.contentPosition(); ok {
			if cmds := projectMarkAt(b.Content, pt, ctx); cmds != nil {
				list.Commands = append(list.Commands, cmds.Commands...)
			}
		}
	}
	list.Add(gfx.PopTransform{})
	return &list
}

func (b *Badge) resolvedPosition() gfx.Point {
	if root := b.base.Parent(); root != nil {
		if pt, ok := anchorPoint(root, b.Host, "bounds-center"); ok {
			return gfx.Point{X: pt.X + b.Offset.X, Y: pt.Y + b.Offset.Y}
		}
	}
	return b.Offset
}

func (b *Badge) contentPosition() (gfx.Point, bool) {
	return gfx.Point{X: b.localBounds().Min.X + b.localBounds().Width()/2, Y: b.localBounds().Min.Y + b.localBounds().Height()/2}, true
}
