package uinav

import (
	"sync"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/theme"
	uirecipe "codeburg.org/lexbit/lurpicui/theme/recipes/uinav"
)

type DrawerMode uint8

const (
	DrawerModal DrawerMode = iota
	DrawerDismissible
	DrawerPermanent
)

type DrawerEdge uint8

const (
	DrawerLeft DrawerEdge = iota
	DrawerRight
	DrawerTop
	DrawerBottom
)

type Drawer struct {
	ID      string
	Mode    DrawerMode
	Edge    DrawerEdge
	Open    store.Binding[bool]
	Content []marks.Mark

	base         facet.Facet
	once         sync.Once
	state        controlState
	layoutRole   *facet.LayoutRole
	viewportRole *facet.ViewportRole
	projection   *facet.ProjectionRole
	hitRole      *facet.HitRole
	inputRole    *facet.InputRole
	focusRole    *facet.FocusRole
}

func init() {
	registerDescriptor(marks.Descriptor{
		Family:            marks.FamilyUINav,
		ConstructionClass: marks.ConstructionComposed,
		Type:              marks.TypeName("uinav:drawer"),
		Focusable:         true,
		HitTestable:       true,
		ChildHosting:      true,
	})
}

func (d *Drawer) Base() *facet.Facet { d.ensureInit(); return &d.base }
func (d *Drawer) Descriptor() marks.Descriptor {
	return marks.Descriptor{Family: marks.FamilyUINav, ConstructionClass: marks.ConstructionComposed, Type: marks.TypeName("uinav:drawer"), Focusable: true, HitTestable: true, ChildHosting: true}
}
func (d *Drawer) AuthoredID() string               { return d.ID }
func (d *Drawer) OnAttach(ctx facet.AttachContext) { d.syncRoles() }
func (d *Drawer) OnDetach()                        {}
func (d *Drawer) OnActivate()                      {}
func (d *Drawer) OnDeactivate()                    {}

func (d *Drawer) ensureInit() {
	d.once.Do(func() {
		ensureBase(&d.base)
		d.base.BindImpl(d)
		d.layoutRole = &facet.LayoutRole{OnMeasure: func(c facet.Constraints) gfx.Size {
			b := d.bounds()
			return gfx.Size{W: b.Width(), H: b.Height()}
		}}
		d.viewportRole = &facet.ViewportRole{Transform: gfx.Identity()}
		d.projection = &facet.ProjectionRole{OnProject: func(ctx facet.ProjectionContext) *gfx.CommandList { return d.project(ctx) }}
		d.hitRole = &facet.HitRole{OnHitTest: func(p gfx.Point) facet.HitResult {
			if d.hitBounds().Contains(p) {
				return facet.HitResult{Hit: true, Cursor: facet.CursorDefault}
			}
			return facet.HitResult{}
		}}
		d.inputRole = &facet.InputRole{
			OnPointer: func(e facet.PointerEvent) bool { return d.handlePointer(e) },
			OnKey:     func(e facet.KeyEvent) bool { return d.handleKey(e) },
		}
		d.focusRole = &facet.FocusRole{
			Focusable:     func() bool { return d.Mode != DrawerPermanent },
			OnFocusGained: func() { d.state.focused = true },
			OnFocusLost:   func() { d.state.focused = false },
		}
		d.base.AddRole(d.layoutRole)
		d.base.AddRole(d.viewportRole)
		d.base.AddRole(d.projection)
		d.base.AddRole(d.hitRole)
		d.base.AddRole(d.inputRole)
		d.base.AddRole(d.focusRole)
		attachChildMarks(&d.base, d.Content)
		d.syncRoles()
	})
}

func (d *Drawer) syncRoles() {
	syncLayout(d.layoutRole, d.bounds())
	syncViewport(d.viewportRole, gfx.Identity())
}

func (d *Drawer) bounds() gfx.Rect {
	return gfx.RectFromXYWH(0, 0, drawerMinWidth(), drawerMaxWidth())
}

func (d *Drawer) hitBounds() gfx.Rect {
	if !d.Open.Get() && d.Mode != DrawerPermanent {
		return gfx.Rect{}
	}
	return d.bounds()
}

func (d *Drawer) surfaceBounds() gfx.Rect {
	b := d.bounds()
	switch d.Edge {
	case DrawerRight:
		return gfx.RectFromXYWH(80, 0, 160, b.Height())
	case DrawerTop:
		return gfx.RectFromXYWH(0, 0, b.Width(), 180)
	case DrawerBottom:
		return gfx.RectFromXYWH(0, 140, b.Width(), 180)
	default:
		return gfx.RectFromXYWH(0, 0, 160, b.Height())
	}
}

func (d *Drawer) entryTransform() gfx.Transform {
	b := d.bounds()
	switch d.Edge {
	case DrawerRight:
		return gfx.Translation(b.Width(), 0)
	case DrawerTop:
		return gfx.Translation(0, -60)
	case DrawerBottom:
		return gfx.Translation(0, 60)
	default:
		return gfx.Translation(-60, 0)
	}
}

func (d *Drawer) handlePointer(e facet.PointerEvent) bool {
	if !d.Open.Get() {
		return false
	}
	if e.Kind == platform.PointerPress {
		if d.Mode != DrawerPermanent && !d.surfaceBounds().Contains(e.Position) {
			if d.Mode != DrawerModal {
				d.Open.Set(false)
			}
			return true
		}
		return true
	}
	return false
}

func (d *Drawer) handleKey(e facet.KeyEvent) bool {
	if d.Mode != DrawerModal || !d.Open.Get() || e.Kind != platform.KeyPress {
		return false
	}
	if e.Key == platform.KeyTab {
		return true
	}
	if e.Key == platform.KeyEscape {
		d.Open.Set(false)
		return true
	}
	return false
}

func (d *Drawer) OnLayerSpecs() []layout.LayerSpec {
	if d.Mode == DrawerPermanent {
		return []layout.LayerSpec{{ID: 1, Placement: layout.PlacementStack, Measurement: layout.MeasureStructural, CoordSpace: layout.CoordParentLayout, HitPolicy: layout.HitNormal, RenderOrder: 1}}
	}
	return []layout.LayerSpec{
		{ID: 1, Placement: layout.PlacementStack, Measurement: layout.MeasureStructural, CoordSpace: layout.CoordParentLayout, HitPolicy: layout.HitBlockBelow, RenderOrder: 100, ClipPolicy: layout.ClipNone},
		{ID: 2, Placement: layout.PlacementFree, Measurement: layout.MeasureNonStructural, CoordSpace: layout.CoordViewport, HitPolicy: layout.HitNormal, RenderOrder: 101, ClipPolicy: layout.ClipToViewport},
	}
}

func (d *Drawer) project(ctx facet.ProjectionContext) *gfx.CommandList {
	slots, _ := uirecipe.ResolveDrawerRecipe(theme.StyleContext{Tokens: theme.DefaultTokens()})
	var list gfx.CommandList
	if d.Mode != DrawerPermanent && d.Open.Get() {
		list.Add(gfx.FillRect{Rect: d.bounds(), Brush: gfx.SolidBrush(fillColor(slots.Scrim.Resolve(theme.StateDefault, theme.DefaultTokens()), gfx.Color{A: 0.4}))})
	}
	if d.Open.Get() || d.Mode == DrawerPermanent {
		surface := d.surfaceBounds()
		list.Add(gfx.PushTransform{Matrix: d.entryTransform()})
		list.Add(gfx.FillRect{Rect: surface, Brush: gfx.SolidBrush(fillColor(slots.Surface.Resolve(theme.StateDefault, theme.DefaultTokens()), gfx.Color{R: 1, G: 1, B: 1, A: 1}))})
		list.Add(gfx.PopTransform{})
	}
	return &list
}
