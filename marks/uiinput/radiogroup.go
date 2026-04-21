package uiinput

import (
	"sync"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/store"
	uirecipe "codeburg.org/lexbit/lurpicui/theme/recipes/uiinput"
	"codeburg.org/lexbit/lurpicui/theme"
)

// RadioOption is one option within a radio group.
type RadioOption struct {
	Key   string
	Label string
}

// RadioGroup is a single-selection option group.
type RadioGroup struct {
	ID       string
	Options  []RadioOption
	Selected store.Binding[string]
	Disabled bool

	base         facet.Facet
	once         sync.Once
	state        controlState
	focusedIndex int
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
		Type:              marks.TypeName("uiinput:radiogroup"),
		Focusable:         true,
		HitTestable:       true,
	})
}

func (r *RadioGroup) Base() *facet.Facet { r.ensureInit(); return &r.base }
func (r *RadioGroup) Descriptor() marks.Descriptor {
	return marks.Descriptor{Family: marks.FamilyUIInput, ConstructionClass: marks.ConstructionComposed, Type: marks.TypeName("uiinput:radiogroup"), Focusable: true, HitTestable: true}
}
func (r *RadioGroup) AuthoredID() string { return r.ID }
func (r *RadioGroup) OnAttach(ctx facet.AttachContext) { r.syncRoles() }
func (r *RadioGroup) OnDetach() {}
func (r *RadioGroup) OnActivate() {}
func (r *RadioGroup) OnDeactivate() {}

func (r *RadioGroup) ensureInit() {
	r.once.Do(func() {
		r.base.BindImpl(r)
		r.layoutRole = &facet.LayoutRole{OnMeasure: func(c facet.Constraints) gfx.Size {
			bounds := r.bounds()
			return gfx.Size{W: bounds.Width(), H: bounds.Height()}
		}}
		r.viewportRole = &facet.ViewportRole{Transform: gfx.Identity()}
		r.projection = &facet.ProjectionRole{OnProject: func(ctx facet.ProjectionContext) *gfx.CommandList { return r.project(ctx) }}
		r.hitRole = &facet.HitRole{OnHitTest: func(p gfx.Point) facet.HitResult {
			if r.bounds().Contains(p) {
				return facet.HitResult{Hit: true, Cursor: facet.CursorPointer}
			}
			return facet.HitResult{}
		}}
		r.inputRole = &facet.InputRole{
			OnKey: func(e facet.KeyEvent) bool { return r.handleKey(e) },
		}
		r.focusRole = &facet.FocusRole{
			Focusable: func() bool { return !r.Disabled },
			OnFocusGained: func() {
				r.state.focused = true
				r.focusedIndex = r.indexOf(r.Selected.Get())
				if r.focusedIndex < 0 {
					r.focusedIndex = 0
				}
			},
			OnFocusLost: func() { r.state.focused = false },
		}
		r.base.AddRole(r.layoutRole)
		r.base.AddRole(r.viewportRole)
		r.base.AddRole(r.projection)
		r.base.AddRole(r.hitRole)
		r.base.AddRole(r.inputRole)
		r.base.AddRole(r.focusRole)
		r.syncRoles()
	})
}

func (r *RadioGroup) syncRoles() {
	r.state.disabled = r.Disabled
	selected := r.indexOf(r.Selected.Get())
	if selected >= 0 {
		r.focusedIndex = selected
	}
}

func (r *RadioGroup) bounds() gfx.Rect {
	return gfx.RectFromXYWH(0, 0, 160, float32(len(r.Options))*28)
}

func (r *RadioGroup) indexOf(key string) int {
	for i, opt := range r.Options {
		if opt.Key == key {
			return i
		}
	}
	return -1
}

func (r *RadioGroup) handleKey(e facet.KeyEvent) bool {
	if r.Disabled || !r.state.focused || e.Kind != platform.KeyPress || len(r.Options) == 0 {
		return false
	}
	switch e.Key {
	case platform.KeyRight, platform.KeyDown:
		r.focusedIndex = (r.focusedIndex + 1) % len(r.Options)
	case platform.KeyLeft, platform.KeyUp:
		r.focusedIndex = (r.focusedIndex - 1 + len(r.Options)) % len(r.Options)
	case platform.KeyEnter, platform.KeySpace:
		r.Selected.Set(r.Options[r.focusedIndex].Key)
	default:
		return false
	}
	r.Selected.Set(r.Options[r.focusedIndex].Key)
	return true
}

func (r *RadioGroup) project(ctx facet.ProjectionContext) *gfx.CommandList {
	slots, _ := uirecipe.ResolveRadioGroupRecipe(theme.StyleContext{Tokens: theme.DefaultTokens()}, uirecipe.RadioGroupStandard)
	var list gfx.CommandList
	for i, opt := range r.Options {
		y := float32(i) * 28
		rect := gfx.RectFromXYWH(0, y, 20, 20)
		if r.Selected.Get() == opt.Key {
			indicator := slots.Indicator.Resolve(r.state.interactionState(), theme.DefaultTokens())
			list.Add(gfx.FillRect{Rect: rect, Brush: gfx.SolidBrush(fillColor(indicator, gfx.Color{A: 1}))})
		} else {
			option := slots.Option.Resolve(r.state.interactionState(), theme.DefaultTokens())
			list.Add(gfx.StrokeRect{Rect: rect, Stroke: gfx.DefaultStroke(1), Brush: gfx.SolidBrush(fillColor(option, gfx.Color{A: 1}))})
		}
	}
	return &list
}
