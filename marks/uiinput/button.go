package uiinput

import (
	"sync"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/annotation"
	"codeburg.org/lexbit/lurpicui/platform"
	uirecipe "codeburg.org/lexbit/lurpicui/theme/recipes/uiinput"
	shared "codeburg.org/lexbit/lurpicui/theme/recipes"
	"codeburg.org/lexbit/lurpicui/theme"
)

// Button is a semantic action control.
type Button struct {
	ID       string
	Label    string
	Icon     *annotation.Icon
	Variant  ButtonVariant
	Disabled bool
	OnPress  func()

	base         facet.Facet
	once         sync.Once
	state        controlState
	action       actionBinding
	layoutRole   *facet.LayoutRole
	viewportRole *facet.ViewportRole
	projection   *facet.ProjectionRole
	hitRole      *facet.HitRole
	inputRole    *facet.InputRole
	focusRole    *facet.FocusRole
}

func init() {
	registerDescriptor(marks.Descriptor{
		Family:            marks.FamilyUIInput,
		ConstructionClass: marks.ConstructionComposed,
		Type:              marks.TypeName("uiinput:button"),
		Focusable:         true,
		HitTestable:       true,
		AnchorExporting:   true,
		ChildHosting:      false,
	})
}

func (b *Button) Base() *facet.Facet { b.ensureInit(); return &b.base }

func (b *Button) Descriptor() marks.Descriptor {
	return marks.Descriptor{
		Family:            marks.FamilyUIInput,
		ConstructionClass: marks.ConstructionComposed,
		Type:              marks.TypeName("uiinput:button"),
		Focusable:         true,
		HitTestable:       true,
		AnchorExporting:   true,
	}
}

func (b *Button) AuthoredID() string { return b.ID }

func (b *Button) OnAttach(ctx facet.AttachContext) {
	b.syncRoles()
}
func (b *Button) OnDetach()     {}
func (b *Button) OnActivate()   {}
func (b *Button) OnDeactivate() {}

func (b *Button) CanFocus() bool { return !b.Disabled }

func (b *Button) ensureInit() {
	b.once.Do(func() {
		b.base.BindImpl(b)
		b.layoutRole = &facet.LayoutRole{OnMeasure: func(c facet.Constraints) gfx.Size {
			bounds := b.bounds()
			return gfx.Size{W: bounds.Width(), H: bounds.Height()}
		}}
		b.viewportRole = &facet.ViewportRole{Transform: gfx.Identity()}
		b.projection = &facet.ProjectionRole{OnProject: func(ctx facet.ProjectionContext) *gfx.CommandList { return b.project(ctx) }}
		b.hitRole = &facet.HitRole{OnHitTest: func(p gfx.Point) facet.HitResult {
			if b.bounds().Contains(p) {
				return facet.HitResult{Hit: true, Cursor: facet.CursorPointer}
			}
			return facet.HitResult{}
		}}
		b.inputRole = &facet.InputRole{
			OnPointer: func(e facet.PointerEvent) bool { return b.handlePointer(e) },
			OnKey:     func(e facet.KeyEvent) bool { return b.handleKey(e) },
		}
		b.focusRole = &facet.FocusRole{
			Focusable: func() bool { return !b.Disabled },
			OnFocusGained: func() {
				b.state.focused = true
			},
			OnFocusLost: func() {
				b.state.focused = false
				b.state.pressed = false
			},
		}
		b.base.AddRole(b.layoutRole)
		b.base.AddRole(b.viewportRole)
		b.base.AddRole(b.projection)
		b.base.AddRole(b.hitRole)
		b.base.AddRole(b.inputRole)
		b.base.AddRole(b.focusRole)
		b.syncRoles()
	})
}

func (b *Button) syncRoles() {
	b.state.disabled = b.Disabled
	syncLayout(b.layoutRole, b.bounds())
	syncViewport(b.viewportRole, gfx.Identity())
}

func (b *Button) bounds() gfx.Rect {
	return gfx.RectFromXYWH(0, 0, 96, 36)
}

func (b *Button) handlePointer(e facet.PointerEvent) bool {
	if b.Disabled {
		return false
	}
	switch e.Kind {
	case platform.PointerPress:
		b.state.pressed = true
		return true
	case platform.PointerRelease:
		wasPressed := b.state.pressed
		b.state.pressed = false
		if wasPressed {
			b.activate()
			return true
		}
	}
	return false
}

func (b *Button) handleKey(e facet.KeyEvent) bool {
	if b.Disabled || !b.state.focused || e.Kind != platform.KeyPress {
		return false
	}
	if e.Key == platform.KeySpace || e.Key == platform.KeyEnter {
		b.activate()
		return true
	}
	return false
}

func (b *Button) activate() {
	if b.Disabled {
		return
	}
	if b.action.OnActivate != nil {
		b.action.OnActivate()
	}
	if b.OnPress != nil {
		b.OnPress()
	}
}

func (b *Button) project(ctx facet.ProjectionContext) *gfx.CommandList {
	slots := b.resolveRecipe()
	bounds := b.bounds()
	state := b.state.interactionState()
	container := slots.Container.Resolve(state, theme.DefaultTokens())
	focus := slots.FocusRing.Resolve(theme.StateFocused, theme.DefaultTokens())
	var list gfx.CommandList
	list.Add(gfx.FillRect{Rect: bounds, Brush: gfx.SolidBrush(fillColor(container, gfx.Color{R: 0.9, G: 0.9, B: 0.92, A: 1}))})
	if b.state.focused && len(focus.Strokes) > 0 {
		stroke := focus.Strokes[0]
		list.Add(gfx.StrokeRect{Rect: bounds.Inset(-2, -2), Stroke: strokeStyle(stroke), Brush: gfx.SolidBrush(strokeColor(focus, gfx.Color{R: 0.3, G: 0.5, B: 1, A: 1}))})
	}
	if b.Label != "" {
		list.Add(gfx.DrawPoints{Points: []gfx.Point{{X: 10, Y: 10}}, Radius: 1, Brush: gfx.SolidBrush(gfx.Color{A: 1})})
	}
	if b.Icon != nil {
		list.Add(gfx.DrawPoints{Points: []gfx.Point{{X: 18, Y: 18}}, Radius: 2, Brush: gfx.SolidBrush(gfx.Color{A: 1})})
	}
	return &list
}

func (b *Button) resolveRecipe() shared.ButtonSlots {
	slots, _ := uirecipe.ResolveButtonRecipe(theme.StyleContext{Tokens: theme.DefaultTokens()}, b.Variant)
	return slots
}
