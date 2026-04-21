package uinav

import (
	"sync"
	"time"

	"codeburg.org/lexbit/lurpicui/animation"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/annotation"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/store"
	uirecipe "codeburg.org/lexbit/lurpicui/theme/recipes/uinav"
	"codeburg.org/lexbit/lurpicui/theme"
)

// TabItem describes one tab.
type TabItem struct {
	Key   string
	Label string
	Icon  *annotation.Icon
}

// Tabs renders a tab strip.
type Tabs struct {
	ID       string
	Items    []TabItem
	Selected store.Binding[string]
	Variant  TabsVariant

	base         facet.Facet
	once         sync.Once
	state        controlState
	layoutRole   *facet.LayoutRole
	viewportRole *facet.ViewportRole
	projection   *facet.ProjectionRole
	hitRole      *facet.HitRole
	inputRole    *facet.InputRole
	focusRole    *facet.FocusRole
	indicator    *animation.AnimatedFloat32
}

func init() {
	registerDescriptor(marks.Descriptor{
		Family:            marks.FamilyUINav,
		ConstructionClass: marks.ConstructionComposed,
		Type:              marks.TypeName("uinav:tabs"),
		Focusable:         true,
		HitTestable:       true,
	})
}

func (t *Tabs) Base() *facet.Facet { t.ensureInit(); return &t.base }
func (t *Tabs) Descriptor() marks.Descriptor {
	return marks.Descriptor{Family: marks.FamilyUINav, ConstructionClass: marks.ConstructionComposed, Type: marks.TypeName("uinav:tabs"), Focusable: true, HitTestable: true}
}
func (t *Tabs) AuthoredID() string { return t.ID }
func (t *Tabs) OnAttach(ctx facet.AttachContext) { t.syncRoles() }
func (t *Tabs) OnDetach() {}
func (t *Tabs) OnActivate() {}
func (t *Tabs) OnDeactivate() {}

func (t *Tabs) ensureInit() {
	t.once.Do(func() {
		t.base.BindImpl(t)
		t.layoutRole = &facet.LayoutRole{OnMeasure: func(c facet.Constraints) gfx.Size {
			b := t.bounds()
			return gfx.Size{W: b.Width(), H: b.Height()}
		}}
		t.viewportRole = &facet.ViewportRole{Transform: gfx.Identity()}
		t.projection = &facet.ProjectionRole{OnProject: func(ctx facet.ProjectionContext) *gfx.CommandList { return t.project(ctx) }}
		t.hitRole = &facet.HitRole{OnHitTest: func(p gfx.Point) facet.HitResult {
			if t.bounds().Contains(p) {
				return facet.HitResult{Hit: true, Cursor: facet.CursorPointer}
			}
			return facet.HitResult{}
		}}
		t.inputRole = &facet.InputRole{
			OnPointer: func(e facet.PointerEvent) bool { return t.handlePointer(e) },
			OnKey:     func(e facet.KeyEvent) bool { return t.handleKey(e) },
		}
		t.focusRole = &facet.FocusRole{
			Focusable: func() bool { return true },
			OnFocusGained: func() { t.state.focused = true },
			OnFocusLost:   func() { t.state.focused = false },
		}
		t.base.AddRole(t.layoutRole)
		t.base.AddRole(t.viewportRole)
		t.base.AddRole(t.projection)
		t.base.AddRole(t.hitRole)
		t.base.AddRole(t.inputRole)
		t.base.AddRole(t.focusRole)
		t.indicator = animation.NewAnimatedValue(func() animation.Float32 {
			return animation.Float32(t.selectedOffset())
		}, animation.TransitionSpec{Duration: 160 * time.Millisecond, Easing: "ease-out"}, nil)
		t.syncRoles()
	})
}

func (t *Tabs) syncRoles() {
	syncLayout(t.layoutRole, t.bounds())
	syncViewport(t.viewportRole, gfx.Identity())
}

func (t *Tabs) bounds() gfx.Rect {
	if len(t.Items) == 0 {
		return rectFromSize(0, 0)
	}
	return gfx.RectFromXYWH(0, 0, float32(len(t.Items))*96, 40)
}

func (t *Tabs) itemBounds(i int) gfx.Rect {
	return gfx.RectFromXYWH(float32(i)*96, 0, 96, 40)
}

func (t *Tabs) indexOf(key string) int {
	for i, item := range t.Items {
		if item.Key == key {
			return i
		}
	}
	return 0
}

func (t *Tabs) selectedOffset() float32 {
	return float32(t.indexOf(t.Selected.Get())) * 96
}

func (t *Tabs) handlePointer(e facet.PointerEvent) bool {
	if e.Kind != platform.PointerPress {
		return false
	}
	for i := range t.Items {
		if t.itemBounds(i).Contains(e.Position) {
			t.Selected.Set(t.Items[i].Key)
			return true
		}
	}
	return false
}

func (t *Tabs) handleKey(e facet.KeyEvent) bool {
	if e.Kind != platform.KeyPress || !t.state.focused || len(t.Items) == 0 {
		return false
	}
	idx := t.indexOf(t.Selected.Get())
	switch e.Key {
	case platform.KeyRight:
		idx = (idx + 1) % len(t.Items)
	case platform.KeyLeft:
		idx = (idx - 1 + len(t.Items)) % len(t.Items)
	default:
		return false
	}
	t.Selected.Set(t.Items[idx].Key)
	return true
}

// Tick advances the animated indicator.
func (t *Tabs) Tick(dt time.Duration) bool {
	t.ensureInit()
	return t.indicator.Tick(dt)
}

func (t *Tabs) project(ctx facet.ProjectionContext) *gfx.CommandList {
	slots, _ := uirecipe.ResolveTabsRecipe(theme.StyleContext{Tokens: theme.DefaultTokens()}, t.Variant)
	var list gfx.CommandList
	for i, item := range t.Items {
		rect := t.itemBounds(i)
		style := slots.Tab.Resolve(theme.StateDefault, theme.DefaultTokens())
		if t.Selected.Get() == item.Key {
			style = slots.Current.Resolve(theme.StateSelected, theme.DefaultTokens())
		}
		list.Add(gfx.FillRect{Rect: rect, Brush: gfx.SolidBrush(fillColor(style, gfx.Color{R: 1, G: 1, B: 1, A: 1}))})
	}
	indicatorX := float32(t.indicator.Current())
	list.Add(gfx.FillRect{Rect: gfx.RectFromXYWH(indicatorX, 38, 96, 2), Brush: gfx.SolidBrush(fillColor(slots.Indicator.Resolve(theme.StateDefault, theme.DefaultTokens()), gfx.Color{R: 0.25, G: 0.45, B: 0.95, A: 1}))})
	return &list
}
