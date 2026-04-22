package uinotification

import (
	"sync"
	"time"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/theme"
	uirecipe "codeburg.org/lexbit/lurpicui/theme/recipes/uinotification"
)

type Snackbar struct {
	ID       string
	Message  string
	Action   *ButtonAction
	Open     store.Binding[bool]
	Duration time.Duration

	base         facet.Facet
	once         sync.Once
	layoutRole   *facet.LayoutRole
	viewportRole *facet.ViewportRole
	projection   *facet.ProjectionRole
	hitRole      *facet.HitRole
	inputRole    *facet.InputRole
	elapsed      time.Duration
}

func init() {
	registerDescriptor(marks.Descriptor{
		Family:            marks.FamilyUINotification,
		ConstructionClass: marks.ConstructionComposed,
		Type:              marks.TypeName("uinotification:snackbar"),
		HitTestable:       true,
	})
}

func (s *Snackbar) Base() *facet.Facet { s.ensureInit(); return &s.base }
func (s *Snackbar) Descriptor() marks.Descriptor {
	return marks.Descriptor{Family: marks.FamilyUINotification, ConstructionClass: marks.ConstructionComposed, Type: marks.TypeName("uinotification:snackbar"), HitTestable: true}
}
func (s *Snackbar) AuthoredID() string               { return s.ID }
func (s *Snackbar) OnAttach(ctx facet.AttachContext) { s.syncRoles() }
func (s *Snackbar) OnDetach()                        {}
func (s *Snackbar) OnActivate()                      {}
func (s *Snackbar) OnDeactivate()                    {}

func (s *Snackbar) ensureInit() {
	s.once.Do(func() {
		s.base.BindImpl(s)
		s.layoutRole = &facet.LayoutRole{OnMeasure: func(c facet.Constraints) gfx.Size {
			b := s.bounds()
			return gfx.Size{W: b.Width(), H: b.Height()}
		}}
		s.viewportRole = &facet.ViewportRole{Transform: gfx.Identity()}
		s.projection = &facet.ProjectionRole{OnProject: func(ctx facet.ProjectionContext) *gfx.CommandList { return s.project(ctx) }}
		s.hitRole = &facet.HitRole{OnHitTest: func(p gfx.Point) facet.HitResult {
			if s.hitBounds().Contains(p) {
				return facet.HitResult{Hit: true, Cursor: facet.CursorPointer}
			}
			return facet.HitResult{}
		}}
		s.inputRole = &facet.InputRole{
			OnPointer: func(e facet.PointerEvent) bool { return s.handlePointer(e) },
		}
		s.base.AddRole(s.layoutRole)
		s.base.AddRole(s.viewportRole)
		s.base.AddRole(s.projection)
		s.base.AddRole(s.hitRole)
		s.base.AddRole(s.inputRole)
		s.syncRoles()
	})
}

func (s *Snackbar) syncRoles() {
	syncLayout(s.layoutRole, s.bounds())
	syncViewport(s.viewportRole, gfx.Identity())
}

func (s *Snackbar) bounds() gfx.Rect {
	w := snackbarWidth()
	if s.Action != nil && s.Action.Label != "" {
		w += 88
	}
	return gfx.RectFromXYWH(0, 0, w, snackbarHeight())
}

func (s *Snackbar) hitBounds() gfx.Rect {
	if !s.Open.Get() {
		return gfx.Rect{}
	}
	return s.bounds()
}

func (s *Snackbar) actionBounds() gfx.Rect {
	b := s.bounds()
	if s.Action == nil {
		return gfx.Rect{}
	}
	return gfx.RectFromXYWH(b.Width()-88, 0, 88, b.Height())
}

func (s *Snackbar) handlePointer(e facet.PointerEvent) bool {
	if !s.Open.Get() || e.Kind != platform.PointerPress {
		return false
	}
	if s.Action != nil && s.actionBounds().Contains(e.Position) {
		s.activateAction()
		return true
	}
	return false
}

func (s *Snackbar) activateAction() {
	if s.Action == nil || s.Action.Disabled {
		return
	}
	if s.Action.OnClick != nil {
		s.Action.OnClick()
	}
	s.Open.Set(false)
}

func (s *Snackbar) OnLayerSpecs() []layout.LayerSpec {
	if !s.Open.Get() {
		return nil
	}
	return []layout.LayerSpec{{
		ID:          1,
		Placement:   layout.PlacementProjected,
		Measurement: layout.MeasureNonStructural,
		CoordSpace:  layout.CoordViewport,
		HitPolicy:   layout.HitBlockBelow,
		RenderOrder: 500,
	}}
}

func (s *Snackbar) project(ctx facet.ProjectionContext) *gfx.CommandList {
	slots, _ := uirecipe.ResolveSnackbarRecipe(theme.StyleContext{Tokens: theme.DefaultTokens()})
	var list gfx.CommandList
	if !s.Open.Get() {
		return &list
	}
	bounds := s.bounds()
	list.Add(gfx.FillRect{Rect: bounds, Brush: gfx.SolidBrush(fillColor(slots.Container.Resolve(theme.StateDefault, theme.DefaultTokens()), gfx.Color{R: 0.15, G: 0.15, B: 0.15, A: 1}))})
	if s.Action != nil && s.Action.Label != "" {
		list.Add(gfx.FillRect{Rect: s.actionBounds(), Brush: gfx.SolidBrush(fillColor(slots.Action.Resolve(theme.StateDefault, theme.DefaultTokens()), gfx.Color{R: 1, G: 1, B: 1, A: 1}))})
	}
	return &list
}

func (s *Snackbar) Tick(dt time.Duration) bool {
	if !s.Open.Get() {
		return false
	}
	if s.Duration <= 0 {
		return dt > 0
	}
	s.elapsed += dt
	if s.elapsed >= s.Duration {
		s.Open.Set(false)
		s.elapsed = 0
		return true
	}
	return dt > 0
}

func (s *Snackbar) ResetTimer() {
	s.elapsed = 0
}
