package structure

import (
	"sync"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
)

// Transform is a semantic variant of Group that emphasizes authored transform intent.
type Transform struct {
	ID       string
	Matrix   gfx.Transform
	Children []marks.Mark

	base           facet.Facet
	once           sync.Once
	layoutRole     *facet.LayoutRole
	viewportRole   *facet.ViewportRole
	projectionRole *facet.ProjectionRole
}

func init() {
	registerStructureDescriptor(marks.Descriptor{
		Family:            marks.FamilyStructure,
		ConstructionClass: marks.ConstructionComposed,
		Type:              marks.TypeName("structure:transform"),
		AnchorExporting:   true,
		ChildHosting:      true,
	})
}

func (t *Transform) Base() *facet.Facet { t.ensureInit(); return &t.base }

func (t *Transform) Descriptor() marks.Descriptor {
	t.ensureInit()
	return marks.Descriptor{
		Family:            marks.FamilyStructure,
		ConstructionClass: marks.ConstructionComposed,
		Type:              marks.TypeName("structure:transform"),
		AnchorExporting:   true,
		ChildHosting:      true,
	}
}

func (t *Transform) AuthoredID() string { return t.ID }
func (t *Transform) OnAttach(ctx facet.AttachContext) {
	t.syncRoles()
}
func (t *Transform) OnDetach()     {}
func (t *Transform) OnActivate()   {}
func (t *Transform) OnDeactivate() {}

func (t *Transform) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	t.ensureInit()
	bounds, ok := unionDescendantBounds(&t.base)
	if !ok {
		return nil
	}
	anchors := boundsAnchors(bounds)
	transform := normaliseTransform(t.Matrix)
	if ctx.Viewport != (layout.Viewport{}) {
		transform = ctx.Viewport.Transform.Multiply(transform)
	}
	return transformAnchors(transform, anchors)
}

func (t *Transform) ensureInit() {
	t.once.Do(func() {
		if t.base.ID() == 0 {
			t.base = facet.NewFacet()
		}
		t.base.BindImpl(t)
		t.layoutRole = &facet.LayoutRole{
			OnMeasure: func(c facet.Constraints) gfx.Size {
				bounds, ok := unionDescendantBounds(&t.base)
				if !ok {
					return gfx.Size{}
				}
				return gfx.Size{W: bounds.Width(), H: bounds.Height()}
			},
		}
		t.viewportRole = &facet.ViewportRole{Transform: normaliseTransform(t.Matrix)}
		t.projectionRole = &facet.ProjectionRole{OnProject: func(ctx facet.ProjectionContext) *gfx.CommandList {
			return &gfx.CommandList{}
		}}
		t.base.AddRole(t.layoutRole)
		t.base.AddRole(t.viewportRole)
		t.base.AddRole(t.projectionRole)
		attachChildMarks(&t.base, t.Children)
		syncLayout(t.layoutRole, t.localBounds())
		syncViewport(t.viewportRole, normaliseTransform(t.Matrix))
	})
}

func (t *Transform) syncRoles() {
	syncLayout(t.layoutRole, t.localBounds())
	syncViewport(t.viewportRole, normaliseTransform(t.Matrix))
}

func (t *Transform) localBounds() gfx.Rect {
	bounds, ok := unionDescendantBounds(&t.base)
	if !ok {
		return gfx.Rect{}
	}
	return bounds
}
