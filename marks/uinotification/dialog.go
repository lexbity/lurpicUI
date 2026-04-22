package uinotification

import (
	"sync"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/theme"
	uirecipe "codeburg.org/lexbit/lurpicui/theme/recipes/uinotification"
)

type Dialog struct {
	ID      string
	Open    store.Binding[bool]
	Title   string
	Body    []marks.Mark
	Actions []marks.Mark
	Variant DialogVariant

	DismissOnEscape   bool
	DismissOnBackdrop bool

	base         facet.Facet
	once         sync.Once
	layoutRole   *facet.LayoutRole
	viewportRole *facet.ViewportRole
	projection   *facet.ProjectionRole
	hitRole      *facet.HitRole
	inputRole    *facet.InputRole
	focusRole    *facet.FocusRole
	focusIndex   int
}

func init() {
	registerDescriptor(marks.Descriptor{
		Family:            marks.FamilyUINotification,
		ConstructionClass: marks.ConstructionComposed,
		Type:              marks.TypeName("uinotification:dialog"),
		Focusable:         true,
		HitTestable:       true,
		ChildHosting:      true,
	})
}

func (d *Dialog) Base() *facet.Facet { d.ensureInit(); return &d.base }
func (d *Dialog) Descriptor() marks.Descriptor {
	return marks.Descriptor{Family: marks.FamilyUINotification, ConstructionClass: marks.ConstructionComposed, Type: marks.TypeName("uinotification:dialog"), Focusable: true, HitTestable: true, ChildHosting: true}
}
func (d *Dialog) AuthoredID() string               { return d.ID }
func (d *Dialog) OnAttach(ctx facet.AttachContext) { d.syncRoles() }
func (d *Dialog) OnDetach()                        {}
func (d *Dialog) OnActivate()                      {}
func (d *Dialog) OnDeactivate()                    {}

func (d *Dialog) ensureInit() {
	d.once.Do(func() {
		d.base.BindImpl(d)
		d.layoutRole = &facet.LayoutRole{OnMeasure: func(c facet.Constraints) gfx.Size {
			b := d.bounds()
			return gfx.Size{W: b.Width(), H: b.Height()}
		}}
		d.viewportRole = &facet.ViewportRole{Transform: gfx.Identity()}
		d.projection = &facet.ProjectionRole{OnProject: func(ctx facet.ProjectionContext) *gfx.CommandList { return d.project(ctx) }}
		d.hitRole = &facet.HitRole{OnHitTest: func(p gfx.Point) facet.HitResult {
			if !d.Open.Get() {
				return facet.HitResult{}
			}
			if d.surfaceBounds().Contains(p) {
				return facet.HitResult{Hit: true, Cursor: facet.CursorDefault}
			}
			return facet.HitResult{Hit: true, Cursor: facet.CursorDefault}
		}}
		d.inputRole = &facet.InputRole{
			OnPointer: func(e facet.PointerEvent) bool { return d.handlePointer(e) },
			OnKey:     func(e facet.KeyEvent) bool { return d.handleKey(e) },
		}
		d.focusRole = &facet.FocusRole{
			Focusable:     func() bool { return d.Open.Get() },
			OnFocusGained: func() { d.focusIndex = 0 },
			OnFocusLost:   func() {},
		}
		d.base.AddRole(d.layoutRole)
		d.base.AddRole(d.viewportRole)
		d.base.AddRole(d.projection)
		d.base.AddRole(d.hitRole)
		d.base.AddRole(d.inputRole)
		d.base.AddRole(d.focusRole)
		attachChildMarks(&d.base, d.Body)
		attachChildMarks(&d.base, d.Actions)
		d.syncRoles()
	})
}

func (d *Dialog) syncRoles() {
	syncLayout(d.layoutRole, d.bounds())
	syncViewport(d.viewportRole, gfx.Identity())
}

func (d *Dialog) bounds() gfx.Rect {
	return gfx.RectFromXYWH(0, 0, dialogWidth(), dialogHeight())
}

func (d *Dialog) surfaceBounds() gfx.Rect {
	b := d.bounds()
	switch d.Variant {
	case DialogFullscreen:
		return gfx.RectFromXYWH(0, 0, 640, 480)
	default:
		return gfx.RectFromXYWH(b.Min.X+24, b.Min.Y+24, b.Width()-48, b.Height()-48)
	}
}

func (d *Dialog) handlePointer(e facet.PointerEvent) bool {
	if !d.Open.Get() || e.Kind != platform.PointerPress {
		return false
	}
	if d.surfaceBounds().Contains(e.Position) {
		return true
	}
	if d.DismissOnBackdrop || (d.Variant == DialogFullscreen && d.DismissOnBackdrop) {
		d.Open.Set(false)
		return true
	}
	return true
}

func (d *Dialog) handleKey(e facet.KeyEvent) bool {
	if !d.Open.Get() || e.Kind != platform.KeyPress {
		return false
	}
	switch e.Key {
	case platform.KeyEscape:
		if d.DismissOnEscape {
			d.Open.Set(false)
		}
		return true
	case platform.KeyTab:
		if len(d.Actions) == 0 {
			return true
		}
		d.focusIndex = (d.focusIndex + 1) % len(d.Actions)
		return true
	default:
		return false
	}
}

func (d *Dialog) OnLayerSpecs() []layout.LayerSpec {
	if !d.Open.Get() {
		return nil
	}
	return []layout.LayerSpec{
		{ID: 1, Placement: layout.PlacementProjected, Measurement: layout.MeasureNonStructural, CoordSpace: layout.CoordViewport, HitPolicy: layout.HitBlockBelow, RenderOrder: 600},
		{ID: 2, Placement: layout.PlacementProjected, Measurement: layout.MeasureNonStructural, CoordSpace: layout.CoordViewport, HitPolicy: layout.HitBlockBelow, RenderOrder: 601},
	}
}

func (d *Dialog) project(ctx facet.ProjectionContext) *gfx.CommandList {
	slots, _ := uirecipe.ResolveDialogRecipe(theme.StyleContext{Tokens: theme.DefaultTokens()}, d.Variant)
	var list gfx.CommandList
	if !d.Open.Get() {
		return &list
	}
	scrim := d.bounds()
	surface := d.surfaceBounds()
	list.Add(gfx.FillRect{Rect: scrim, Brush: gfx.SolidBrush(fillColor(slots.Scrim.Resolve(theme.StateDefault, theme.DefaultTokens()), gfx.Color{A: 0.4}))})
	list.Add(gfx.FillRect{Rect: surface, Brush: gfx.SolidBrush(fillColor(slots.Surface.Resolve(theme.StateDefault, theme.DefaultTokens()), gfx.Color{R: 1, G: 1, B: 1, A: 1}))})
	return &list
}
