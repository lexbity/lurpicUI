package structure

import (
	"sync"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
)

// ClipShape selects the clipping geometry used by Clip.
type ClipShape uint8

const (
	ClipRect ClipShape = iota
	ClipRoundedRect
	ClipPathShape
)

// AnchorSourceRef identifies a mark/anchor pair forwarded by future structure marks.
type AnchorSourceRef struct {
	MarkID string
	Anchor string
}

// Clip constrains descendant rendering and hit testing to a local clipping region.
type Clip struct {
	ID       string
	Shape    ClipShape
	Bounds   gfx.Rect
	Radius   float32
	Path     *gfx.Path
	Children []marks.Mark

	base         facet.Facet
	once         sync.Once
	layoutRole   *facet.LayoutRole
	viewportRole *facet.ViewportRole
}

func init() {
	registerStructureDescriptor(marks.Descriptor{
		Family:            marks.FamilyStructure,
		ConstructionClass: marks.ConstructionComposed,
		Type:              marks.TypeName("structure:clip"),
		AnchorExporting:   true,
		ChildHosting:      true,
	})
}

func (c *Clip) Base() *facet.Facet { c.ensureInit(); return &c.base }

func (c *Clip) Descriptor() marks.Descriptor {
	c.ensureInit()
	return marks.Descriptor{
		Family:            marks.FamilyStructure,
		ConstructionClass: marks.ConstructionComposed,
		Type:              marks.TypeName("structure:clip"),
		AnchorExporting:   true,
		ChildHosting:      true,
	}
}

func (c *Clip) AuthoredID() string { return c.ID }
func (c *Clip) OnAttach(ctx facet.AttachContext) {
	c.syncRoles()
}
func (c *Clip) OnDetach()     {}
func (c *Clip) OnActivate()   {}
func (c *Clip) OnDeactivate() {}

func (c *Clip) OnLayerSpecs() []layout.LayerSpec {
	bounds := c.localClipBounds()
	if bounds.IsEmpty() {
		return nil
	}
	return []layout.LayerSpec{{
		ID:          1,
		Placement:   layout.PlacementStack,
		Measurement: layout.MeasureStructural,
		CoordSpace:  layout.CoordParentLayout,
		CoordLimits: layout.CoordLimits{Bounds: bounds},
		HitPolicy:   layout.HitNormal,
		RenderOrder: 0,
		ClipPolicy:  layout.ClipToContent,
	}}
}

func (c *Clip) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	c.ensureInit()
	bounds := c.localClipBounds()
	if bounds.IsEmpty() {
		return nil
	}
	transform := gfx.Identity()
	if ctx.Viewport != (layout.Viewport{}) {
		transform = ctx.Viewport.Transform
	}
	return transformAnchors(transform, boundsAnchors(bounds))
}

func (c *Clip) ensureInit() {
	c.once.Do(func() {
		if c.base.ID() == 0 {
			c.base = facet.NewFacet()
		}
		c.base.BindImpl(c)
		c.layoutRole = &facet.LayoutRole{
			OnMeasure: func(cn facet.Constraints) gfx.Size {
				bounds := c.localClipBounds()
				return gfx.Size{W: bounds.Width(), H: bounds.Height()}
			},
		}
		c.viewportRole = &facet.ViewportRole{Transform: gfx.Identity()}
		c.base.AddRole(c.layoutRole)
		c.base.AddRole(c.viewportRole)
		attachChildMarks(&c.base, c.Children)
		syncLayout(c.layoutRole, c.localClipBounds())
		syncViewport(c.viewportRole, gfx.Identity())
	})
}

func (c *Clip) syncRoles() {
	syncLayout(c.layoutRole, c.localClipBounds())
	syncViewport(c.viewportRole, gfx.Identity())
}

func (c *Clip) localClipBounds() gfx.Rect {
	switch c.Shape {
	case ClipPathShape:
		if c.Path != nil {
			if b := pathBounds(*c.Path); !b.IsEmpty() {
				return b
			}
		}
		return c.Bounds
	default:
		return c.Bounds
	}
}
