package uiinput

import (
	"sync"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/annotation"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/text"
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
	Theme    theme.Context
	Shaper   *text.Shaper

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
		ensureBase(&b.base)
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
				invalidate(&b.base, facet.DirtyProjection, "button-focus")
			},
			OnFocusLost: func() {
				b.state.focused = false
				b.state.pressed = false
				invalidate(&b.base, facet.DirtyProjection, "button-focus")
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
	invalidate(&b.base, facet.DirtyProjection, "button-sync")
}

func (b *Button) bounds() gfx.Rect {
	if b.layoutRole != nil && !b.layoutRole.ArrangedBounds.IsEmpty() {
		return b.layoutRole.ArrangedBounds
	}
	return gfx.RectFromXYWH(0, 0, 96, buttonHeight())
}

func (b *Button) handlePointer(e facet.PointerEvent) bool {
	if b.Disabled {
		return false
	}
	switch e.Kind {
	case platform.PointerPress:
		b.state.hovered = true
		b.state.pressed = true
		invalidate(&b.base, facet.DirtyProjection, "button-press")
		return true
	case platform.PointerEnter:
		b.state.hovered = true
		invalidate(&b.base, facet.DirtyProjection, "button-hover")
		return true
	case platform.PointerMove:
		b.state.hovered = true
		invalidate(&b.base, facet.DirtyProjection, "button-hover")
		return true
	case platform.PointerLeave:
		b.state.hovered = false
		b.state.pressed = false
		invalidate(&b.base, facet.DirtyProjection, "button-hover")
		return true
	case platform.PointerRelease:
		wasPressed := b.state.pressed
		b.state.pressed = false
		invalidate(&b.base, facet.DirtyProjection, "button-release")
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
		invalidate(&b.base, facet.DirtyProjection, "button-activate")
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
	_ = ctx
	th := b.themeContext()
	bounds := b.bounds()
	var list gfx.CommandList
	bg := th.Color(theme.ColorSurfaceVariant)
	fg := th.Color(theme.ColorText)
	switch b.Variant {
	case ButtonFilled:
		bg = th.Color(theme.ColorPrimary)
		fg = th.Color(theme.ColorOnPrimary)
	case ButtonOutlined:
		bg = th.Color(theme.ColorSurface)
	case ButtonTonal:
		bg = th.Color(theme.ColorSurfaceVariant)
		fg = th.Color(theme.ColorText)
	case ButtonText:
		bg = gfx.Color{}
	default:
		bg = th.Color(theme.ColorSurfaceVariant)
	}
	if bg.A == 0 {
		bg = gfx.Color{R: 0.16, G: 0.17, B: 0.2, A: 1}
	}
	if b.state.hovered && !b.state.pressed {
		bg = gfx.Color{
			R: float32(clampFloat(float64(bg.R)+0.04, 0, 1)),
			G: float32(clampFloat(float64(bg.G)+0.04, 0, 1)),
			B: float32(clampFloat(float64(bg.B)+0.04, 0, 1)),
			A: bg.A,
		}
	}
	if b.state.pressed {
		bg = gfx.Color{
			R: float32(clampFloat(float64(bg.R)+0.08, 0, 1)),
			G: float32(clampFloat(float64(bg.G)+0.08, 0, 1)),
			B: float32(clampFloat(float64(bg.B)+0.08, 0, 1)),
			A: bg.A,
		}
	}
	list.Add(gfx.FillRect{Rect: bounds, Brush: gfx.SolidBrush(bg)})
	list.Add(gfx.StrokeRect{Rect: bounds, Brush: gfx.SolidBrush(th.Color(theme.ColorBorder))})
	if b.state.focused {
		list.Add(gfx.StrokeRect{Rect: bounds.Inset(-2, -2), Brush: gfx.SolidBrush(th.Color(theme.ColorPrimary))})
	}
	if b.Label != "" {
		labelStyle := th.TextStyle(theme.TextLabelS)
		if b.Shaper != nil {
			layout := b.Shaper.ShapeSimple(b.Label, labelStyle)
			if layout != nil {
				x := bounds.Min.X + (bounds.Width()-layout.Bounds.Width())/2
				y := bounds.Min.Y + (bounds.Height()-layout.Bounds.Height())/2
				drawText(&list, b.Shaper, x, y, b.Label, labelStyle, fg)
			}
		}
	}
	if b.Icon != nil {
		list.Add(gfx.DrawPoints{Points: []gfx.Point{{X: bounds.Min.X + 16, Y: bounds.Min.Y + bounds.Height()/2}}, Radius: 2, Brush: gfx.SolidBrush(fg)})
	}
	return &list
}

func (b *Button) themeContext() theme.Context {
	if b.Theme != nil {
		return b.Theme
	}
	return theme.Default()
}
