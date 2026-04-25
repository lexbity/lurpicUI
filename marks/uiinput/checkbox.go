package uiinput

import (
	"sync"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/interaction"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/theme"
	uirecipe "codeburg.org/lexbit/lurpicui/theme/recipes/uiinput"
)

// Checkbox is a boolean toggle control.
type Checkbox struct {
	ID       string
	Checked  store.Binding[bool]
	Label    string
	Disabled bool

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
		Family:            marks.FamilyUIInput,
		ConstructionClass: marks.ConstructionComposed,
		Type:              marks.TypeName("uiinput:checkbox"),
		Focusable:         true,
		HitTestable:       true,
	})
}

func (c *Checkbox) Base() *facet.Facet { c.ensureInit(); return &c.base }
func (c *Checkbox) Descriptor() marks.Descriptor {
	return marks.Descriptor{Family: marks.FamilyUIInput, ConstructionClass: marks.ConstructionComposed, Type: marks.TypeName("uiinput:checkbox"), Focusable: true, HitTestable: true}
}
func (c *Checkbox) AuthoredID() string               { return c.ID }
func (c *Checkbox) OnAttach(ctx facet.AttachContext) { c.syncRoles() }
func (c *Checkbox) OnDetach()                        {}
func (c *Checkbox) OnActivate()                      {}
func (c *Checkbox) OnDeactivate()                    {}

func (c *Checkbox) ensureInit() {
	c.once.Do(func() {
		ensureBase(&c.base)
		c.base.BindImpl(c)
		c.layoutRole = &facet.LayoutRole{OnMeasure: func(cn facet.Constraints) gfx.Size {
			bounds := c.bounds()
			return gfx.Size{W: bounds.Width(), H: bounds.Height()}
		}}
		c.viewportRole = &facet.ViewportRole{Transform: gfx.Identity()}
		c.projection = &facet.ProjectionRole{OnProject: func(ctx facet.ProjectionContext) *gfx.CommandList { return c.project(ctx) }}
		c.hitRole = &facet.HitRole{OnHitTest: func(p gfx.Point) facet.HitResult {
			if c.bounds().Contains(p) {
				return facet.HitResult{Hit: true, Cursor: facet.CursorPointer}
			}
			return facet.HitResult{}
		}}
		c.inputRole = &facet.InputRole{
			OnPointer: func(e facet.PointerEvent) bool { return c.handlePointer(e) },
			OnKey:     func(e facet.KeyEvent) bool { return c.handleKey(e) },
		}
		c.focusRole = &facet.FocusRole{
			Focusable:     func() bool { return !c.Disabled },
			OnFocusGained: func() { c.state.focused = true },
			OnFocusLost:   func() { c.state.focused = false },
		}
		c.base.AddRole(c.layoutRole)
		c.base.AddRole(c.viewportRole)
		c.base.AddRole(c.projection)
		c.base.AddRole(c.hitRole)
		c.base.AddRole(c.inputRole)
		c.base.AddRole(c.focusRole)
		c.syncRoles()
	})
}

func (c *Checkbox) syncRoles() {
	c.state.disabled = c.Disabled
}

func (c *Checkbox) bounds() gfx.Rect { return gfx.RectFromXYWH(0, 0, checkboxSize(), checkboxSize()) }

func (c *Checkbox) handlePointer(e facet.PointerEvent) bool {
	if c.Disabled {
		return false
	}
	if interaction.TogglePressReleaseState(&c.state.pressed, c.Disabled, e, c.toggle) {
		return true
	}
	return false
}

func (c *Checkbox) handleKey(e facet.KeyEvent) bool {
	if c.Disabled || !c.state.focused || e.Kind != platform.KeyPress {
		return false
	}
	if e.Key == platform.KeyEnter || e.Key == platform.KeySpace {
		c.toggle()
		return true
	}
	return false
}

func (c *Checkbox) toggle() {
	c.Checked.Set(!c.Checked.Get())
}

func (c *Checkbox) project(ctx facet.ProjectionContext) *gfx.CommandList {
	slots, _ := uirecipe.ResolveCheckboxRecipe(theme.StyleContext{Tokens: theme.DefaultTokens()}, uirecipe.CheckboxStandard)
	state := c.state.interactionState()
	var list gfx.CommandList
	bounds := c.bounds()
	box := slots.Box.Resolve(state, theme.DefaultTokens())
	check := slots.Check.Resolve(state, theme.DefaultTokens())
	list.Add(gfx.FillRect{Rect: bounds, Brush: gfx.SolidBrush(fillColor(box, gfx.Color{R: 1, G: 1, B: 1, A: 1}))})
	if c.Checked.Get() {
		list.Add(gfx.FillRect{Rect: bounds.Inset(6, 6), Brush: gfx.SolidBrush(fillColor(check, gfx.Color{R: 0.2, G: 0.45, B: 0.9, A: 1}))})
	}
	if c.state.focused {
		focus := slots.FocusRing.Resolve(theme.StateFocused, theme.DefaultTokens())
		if len(focus.Strokes) > 0 {
			list.Add(gfx.StrokeRect{Rect: bounds.Inset(-2, -2), Stroke: strokeStyle(focus.Strokes[0]), Brush: gfx.SolidBrush(strokeColor(focus, gfx.Color{R: 0.3, G: 0.5, B: 1, A: 1}))})
		}
	}
	return &list
}
