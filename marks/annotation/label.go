package annotation

import (
	"sync"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/basic"
)

// LabelPlacement controls how a label is positioned relative to an anchor.
type LabelPlacement uint8

const (
	LabelFree LabelPlacement = iota
	LabelAnchorAttached
)

// Label is a reusable text annotation with optional body decoration.
type Label struct {
	ID         string
	Text       basic.Text
	Placement  LabelPlacement
	AnchorRef  *AnchorSourceRef
	Padding    gfx.Insets
	Background bool
	Halo       bool
	Offset     gfx.Point

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
		Type:              marks.TypeName("annotation:label"),
		HitTestable:       true,
		AnchorExporting:   true,
		ChildHosting:      false,
	})
}

func (l *Label) Base() *facet.Facet { l.ensureInit(); return &l.base }

func (l *Label) Descriptor() marks.Descriptor {
	return marks.Descriptor{
		Family:            marks.FamilyAnnotation,
		ConstructionClass: marks.ConstructionComposed,
		Type:              marks.TypeName("annotation:label"),
		HitTestable:       true,
		AnchorExporting:   true,
	}
}

func (l *Label) AuthoredID() string { return l.ID }
func (l *Label) OnAttach(ctx facet.AttachContext) {
	l.syncRoles()
}
func (l *Label) OnDetach()     {}
func (l *Label) OnActivate()   {}
func (l *Label) OnDeactivate() {}

func (l *Label) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	l.ensureInit()
	bounds := l.localBounds()
	if bounds.IsEmpty() {
		return nil
	}
	anchors := layout.AnchorSet{
		"text-center":   {X: bounds.Min.X + bounds.Width()/2, Y: bounds.Min.Y + bounds.Height()/2},
		"text-baseline": {X: bounds.Min.X, Y: bounds.Max.Y},
		"body-center":   {X: bounds.Min.X + bounds.Width()/2, Y: bounds.Min.Y + bounds.Height()/2},
		"bounds-center": {X: bounds.Min.X + bounds.Width()/2, Y: bounds.Min.Y + bounds.Height()/2},
	}
	transform := gfx.Translation(l.resolvedPosition().X, l.resolvedPosition().Y)
	if ctx.Viewport != (layout.Viewport{}) {
		transform = ctx.Viewport.Transform.Multiply(transform)
	}
	return transformAnchors(transform, anchors)
}

func (l *Label) HitTest(world gfx.Point) bool {
	l.ensureInit()
	inv, ok := gfx.Translation(l.resolvedPosition().X, l.resolvedPosition().Y).Inverse()
	if !ok {
		return false
	}
	local := inv.TransformPoint(world)
	return l.localBounds().Contains(local)
}

func (l *Label) ensureInit() {
	l.once.Do(func() {
		l.base.BindImpl(l)
		l.layoutRole = &facet.LayoutRole{
			OnMeasure: func(c facet.Constraints) gfx.Size {
				bounds := l.localBounds()
				return gfx.Size{W: bounds.Width(), H: bounds.Height()}
			},
		}
		l.viewportRole = &facet.ViewportRole{Transform: gfx.Identity()}
		l.projection = &facet.ProjectionRole{OnProject: func(ctx facet.ProjectionContext) *gfx.CommandList {
			return l.project(ctx)
		}}
		l.hitRole = &facet.HitRole{OnHitTest: func(p gfx.Point) facet.HitResult {
			if l.localBounds().Contains(p) {
				return facet.HitResult{Hit: true, Cursor: facet.CursorDefault}
			}
			return facet.HitResult{}
		}}
		l.base.AddRole(l.layoutRole)
		l.base.AddRole(l.viewportRole)
		l.base.AddRole(l.projection)
		l.base.AddRole(l.hitRole)
		syncLayout(l.layoutRole, l.localBounds())
		syncViewport(l.viewportRole, gfx.Identity())
	})
}

func (l *Label) syncRoles() {
	syncLayout(l.layoutRole, l.localBounds())
	syncViewport(l.viewportRole, gfx.Identity())
}

func (l *Label) localBounds() gfx.Rect {
	textBounds := textMarkBounds(&l.Text, gfx.Identity())
	padding := l.Padding
	return gfx.RectFromXYWH(
		textBounds.Min.X-padding.Left,
		textBounds.Min.Y-padding.Top,
		textBounds.Width()+padding.Left+padding.Right,
		textBounds.Height()+padding.Top+padding.Bottom,
	)
}

func (l *Label) project(ctx facet.ProjectionContext) *gfx.CommandList {
	bounds := l.localBounds()
	if bounds.IsEmpty() {
		return &gfx.CommandList{}
	}
	var list gfx.CommandList
	list.Add(gfx.PushTransform{Matrix: gfx.Translation(l.resolvedPosition().X, l.resolvedPosition().Y)})
	if l.Background {
		list.Add(gfx.FillRect{Rect: bounds, Brush: gfx.SolidBrush(gfx.Color{A: 0.12})})
	}
	if l.Halo {
		list.Add(gfx.StrokeRect{Rect: bounds.Inset(2, 2), Stroke: gfx.DefaultStroke(1), Brush: gfx.SolidBrush(gfx.Color{A: 0.18})})
	}
	textTx := gfx.Translation(bounds.Min.X+l.Padding.Left, bounds.Min.Y+l.Padding.Top)
	if textCmds := textMarkCommandList(&l.Text, textTx); textCmds != nil {
		list.Commands = append(list.Commands, textCmds.Commands...)
	}
	list.Add(gfx.PopTransform{})
	return &list
}

func (l *Label) resolvedPosition() gfx.Point {
	if l.Placement == LabelAnchorAttached && l.AnchorRef != nil {
		if root := l.base.Parent(); root != nil {
			if pt, ok := anchorPoint(root, *l.AnchorRef, "bounds-center"); ok {
				return gfx.Point{X: pt.X + l.Offset.X, Y: pt.Y + l.Offset.Y}
			}
		}
	}
	return l.Offset
}
